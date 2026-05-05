package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseURL = func() string {
		if v := os.Getenv("SNAPAH_URL"); v != "" {
			return v
		}
		return "http://localhost:8082"
	}()

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#f5c800")).
			MarginBottom(1)

	styleBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22a64a"))

	styleSelected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f5c800")).
			Bold(true)

	styleMuted = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666688"))

	styleSuccess = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22a64a"))

	styleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e8360a"))

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2a2a4a")).
			Padding(0, 1)
)

// ── State machine ────────────────────────────────────────

type screen int

const (
	screenLogin screen = iota
	screenMenu
	screenNodes
	screenSnapshots
	screenEvents
	screenPolicies
)

type model struct {
	screen  screen
	token   string
	user    string
	role    string
	msg     string
	err     string
	loading bool
	cursor  int

	// login
	inputs  []textinput.Model
	focused int

	// tables
	nodesTable     table.Model
	snapshotsTable table.Model
	eventsTable    table.Model
	policiesTable  table.Model

	// data
	width  int
	height int
}

// ── Messages ─────────────────────────────────────────────

type loginDoneMsg struct{ token, user, role string }
type errMsg struct{ err string }
type nodesDoneMsg struct{ rows []table.Row }
type snapsDoneMsg struct{ rows []table.Row }
type eventsDoneMsg struct{ rows []table.Row }
type policiesDoneMsg struct{ rows []table.Row }

// ── Init ─────────────────────────────────────────────────

func initialModel() model {
	u := textinput.New()
	u.Placeholder = "admin"
	u.Focus()
	u.Width = 30

	p := textinput.New()
	p.Placeholder = "contraseña"
	p.EchoMode = textinput.EchoPassword
	p.Width = 30

	return model{
		screen: screenLogin,
		inputs: []textinput.Model{u, p},
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// ── Update ────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch m.screen {
		case screenLogin:
			return m.updateLogin(msg)
		case screenMenu:
			return m.updateMenu(msg)
		case screenNodes, screenSnapshots, screenEvents, screenPolicies:
			return m.updateTable(msg)
		}

	case loginDoneMsg:
		m.loading = false
		m.token = msg.token
		m.user = msg.user
		m.role = msg.role
		m.screen = screenMenu
		m.cursor = 0
		m.err = ""

	case nodesDoneMsg:
		m.loading = false
		cols := []table.Column{
			{Title: "ID", Width: 10},
			{Title: "Hostname", Width: 20},
			{Title: "Dirección", Width: 22},
			{Title: "Estado", Width: 10},
			{Title: "Último visto", Width: 20},
		}
		t := table.New(table.WithColumns(cols), table.WithRows(msg.rows),
			table.WithFocused(true), table.WithHeight(10))
		t.SetStyles(tableStyles())
		m.nodesTable = t
		m.screen = screenNodes

	case snapsDoneMsg:
		m.loading = false
		cols := []table.Column{
			{Title: "ID", Width: 10},
			{Title: "Path", Width: 30},
			{Title: "RO", Width: 6},
			{Title: "Estado", Width: 10},
			{Title: "Creado", Width: 20},
		}
		t := table.New(table.WithColumns(cols), table.WithRows(msg.rows),
			table.WithFocused(true), table.WithHeight(10))
		t.SetStyles(tableStyles())
		m.snapshotsTable = t
		m.screen = screenSnapshots

	case eventsDoneMsg:
		m.loading = false
		cols := []table.Column{
			{Title: "Tipo", Width: 22},
			{Title: "Mensaje", Width: 40},
			{Title: "Severity", Width: 10},
			{Title: "Hora", Width: 20},
		}
		t := table.New(table.WithColumns(cols), table.WithRows(msg.rows),
			table.WithFocused(true), table.WithHeight(10))
		t.SetStyles(tableStyles())
		m.eventsTable = t
		m.screen = screenEvents

	case policiesDoneMsg:
		m.loading = false
		cols := []table.Column{
			{Title: "Nombre", Width: 18},
			{Title: "Schedule", Width: 14},
			{Title: "Path", Width: 24},
			{Title: "Retención", Width: 12},
			{Title: "Activa", Width: 8},
		}
		t := table.New(table.WithColumns(cols), table.WithRows(msg.rows),
			table.WithFocused(true), table.WithHeight(10))
		t.SetStyles(tableStyles())
		m.policiesTable = t
		m.screen = screenPolicies

	case errMsg:
		m.loading = false
		m.err = msg.err
	}

	return m, nil
}

func (m model) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyTab, tea.KeyDown:
		m.focused = (m.focused + 1) % len(m.inputs)
		for i := range m.inputs {
			if i == m.focused {
				m.inputs[i].Focus()
			} else {
				m.inputs[i].Blur()
			}
		}
	case tea.KeyEnter:
		if m.focused == 0 {
			m.focused = 1
			m.inputs[0].Blur()
			m.inputs[1].Focus()
		} else {
			m.loading = true
			m.err = ""
			user := m.inputs[0].Value()
			pass := m.inputs[1].Value()
			return m, doLogin(user, pass)
		}
	default:
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}
	return m, nil
}

var menuItems = []string{
	"🖥️  Nodos",
	"📸  Snapshots",
	"📋  Eventos",
	"📅  Políticas",
	"❌  Salir",
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(menuItems)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		switch m.cursor {
		case 0:
			m.loading = true
			return m, fetchNodes(m.token)
		case 1:
			m.loading = true
			return m, fetchSnapshots(m.token)
		case 2:
			m.loading = true
			return m, fetchEvents(m.token)
		case 3:
			m.loading = true
			return m, fetchPolicies(m.token)
		case 4:
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyBackspace:
		m.screen = screenMenu
		m.cursor = 0
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	var cmd tea.Cmd
	switch m.screen {
	case screenNodes:
		m.nodesTable, cmd = m.nodesTable.Update(msg)
	case screenSnapshots:
		m.snapshotsTable, cmd = m.snapshotsTable.Update(msg)
	case screenEvents:
		m.eventsTable, cmd = m.eventsTable.Update(msg)
	case screenPolicies:
		m.policiesTable, cmd = m.policiesTable.Update(msg)
	}
	return m, cmd
}

// ── View ──────────────────────────────────────────────────

func (m model) View() string {
	header := styleTitle.Render("snapah pow") + "  " +
		styleBar.Render("▬▬▬") + "\n"

	if m.loading {
		return header + styleMuted.Render("  cargando...") + "\n"
	}

	var body string
	switch m.screen {
	case screenLogin:
		body = m.viewLogin()
	case screenMenu:
		body = m.viewMenu()
	case screenNodes:
		body = m.viewTable("Nodos", m.nodesTable)
	case screenSnapshots:
		body = m.viewTable("Snapshots", m.snapshotsTable)
	case screenEvents:
		body = m.viewTable("Eventos", m.eventsTable)
	case screenPolicies:
		body = m.viewTable("Políticas", m.policiesTable)
	}

	footer := ""
	if m.err != "" {
		footer = "\n" + styleError.Render("  ✗ "+m.err)
	}
	if m.msg != "" {
		footer = "\n" + styleSuccess.Render("  ✓ "+m.msg)
	}

	return header + body + footer + "\n"
}

func (m model) viewLogin() string {
	s := "\n"
	s += "  " + styleMuted.Render("Usuario") + "\n"
	s += "  " + m.inputs[0].View() + "\n\n"
	s += "  " + styleMuted.Render("Contraseña") + "\n"
	s += "  " + m.inputs[1].View() + "\n\n"
	s += "  " + styleMuted.Render("Tab/↓ cambiar campo  ·  Enter iniciar sesión  ·  Esc salir") + "\n"
	return styleBox.Render(s)
}

func (m model) viewMenu() string {
	s := "\n  " + styleMuted.Render("Conectado como ") +
		styleSelected.Render(m.user) + " (" + m.role + ")\n\n"
	for i, item := range menuItems {
		if i == m.cursor {
			s += "  " + styleSelected.Render("▶ "+item) + "\n"
		} else {
			s += "    " + styleMuted.Render(item) + "\n"
		}
	}
	s += "\n  " + styleMuted.Render("↑/↓ navegar  ·  Enter seleccionar  ·  Esc salir") + "\n"
	return styleBox.Render(s)
}

func (m model) viewTable(title string, t table.Model) string {
	s := styleTitle.Render("  "+title) + "\n"
	s += t.View() + "\n"
	s += styleMuted.Render("  ↑/↓ navegar  ·  Esc volver al menú") + "\n"
	return s
}

// ── API calls ─────────────────────────────────────────────

func doLogin(user, pass string) tea.Cmd {
	return func() tea.Msg {
		body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
		resp, err := http.Post(baseURL+"/api/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			return errMsg{"no se pudo conectar: " + err.Error()}
		}
		defer resp.Body.Close()
		var data map[string]string
		json.NewDecoder(resp.Body).Decode(&data)
		if resp.StatusCode != 200 {
			return errMsg{data["error"]}
		}
		return loginDoneMsg{data["token"], data["username"], data["role"]}
	}
}

func apiGet(token, path string) ([]byte, error) {
	req, _ := http.NewRequest("GET", baseURL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func fetchNodes(token string) tea.Cmd {
	return func() tea.Msg {
		data, err := apiGet(token, "/api/nodes")
		if err != nil {
			return errMsg{err.Error()}
		}
		var resp struct {
			Nodes []struct {
				ID       string    `json:"id"`
				Hostname string    `json:"hostname"`
				Address  string    `json:"address"`
				Status   string    `json:"status"`
				LastSeen time.Time `json:"last_seen"`
			} `json:"nodes"`
		}
		json.Unmarshal(data, &resp)
		var rows []table.Row
		for _, n := range resp.Nodes {
			id := n.ID
			if len(id) > 8 {
				id = id[:8] + "..."
			}
			rows = append(rows, table.Row{
				id, n.Hostname, n.Address, n.Status,
				n.LastSeen.Format("02/01 15:04:05"),
			})
		}
		if len(rows) == 0 {
			rows = append(rows, table.Row{"—", "Sin nodos", "", "", ""})
		}
		return nodesDoneMsg{rows}
	}
}

func fetchSnapshots(token string) tea.Cmd {
	return func() tea.Msg {
		data, err := apiGet(token, "/api/snapshots")
		if err != nil {
			return errMsg{err.Error()}
		}
		var resp struct {
			Snapshots []struct {
				ID           string    `json:"id"`
				SnapshotPath string    `json:"snapshot_path"`
				IsReadOnly   bool      `json:"is_readonly"`
				Status       string    `json:"status"`
				CreatedAt    time.Time `json:"created_at"`
			} `json:"snapshots"`
		}
		json.Unmarshal(data, &resp)
		var rows []table.Row
		for _, s := range resp.Snapshots {
			id := s.ID
			if len(id) > 8 {
				id = id[:8] + "..."
			}
			ro := "RW"
			if s.IsReadOnly {
				ro = "RO"
			}
			rows = append(rows, table.Row{
				id, s.SnapshotPath, ro, s.Status,
				s.CreatedAt.Format("02/01 15:04:05"),
			})
		}
		if len(rows) == 0 {
			rows = append(rows, table.Row{"—", "Sin snapshots", "", "", ""})
		}
		return snapsDoneMsg{rows}
	}
}

func fetchEvents(token string) tea.Cmd {
	return func() tea.Msg {
		data, err := apiGet(token, "/api/events")
		if err != nil {
			return errMsg{err.Error()}
		}
		var resp struct {
			Events []struct {
				Type      string    `json:"type"`
				Message   string    `json:"message"`
				Severity  string    `json:"severity"`
				CreatedAt time.Time `json:"created_at"`
			} `json:"events"`
		}
		json.Unmarshal(data, &resp)
		var rows []table.Row
		for _, e := range resp.Events {
			msg := e.Message
			if len(msg) > 38 {
				msg = msg[:38] + "…"
			}
			rows = append(rows, table.Row{
				e.Type, msg, e.Severity,
				e.CreatedAt.Format("02/01 15:04:05"),
			})
		}
		if len(rows) == 0 {
			rows = append(rows, table.Row{"—", "Sin eventos", "", ""})
		}
		return eventsDoneMsg{rows}
	}
}

func fetchPolicies(token string) tea.Cmd {
	return func() tea.Msg {
		data, err := apiGet(token, "/api/policies")
		if err != nil {
			return errMsg{err.Error()}
		}
		var resp struct {
			Policies []struct {
				Name          string `json:"name"`
				Schedule      string `json:"schedule"`
				SubvolumePath string `json:"subvolume_path"`
				RetentionDaily int   `json:"retention_daily"`
				RetentionWeekly int  `json:"retention_weekly"`
				Enabled       bool   `json:"enabled"`
			} `json:"policies"`
		}
		json.Unmarshal(data, &resp)
		var rows []table.Row
		for _, p := range resp.Policies {
			active := "no"
			if p.Enabled {
				active = "sí"
			}
			ret := fmt.Sprintf("%dd/%dw", p.RetentionDaily, p.RetentionWeekly)
			rows = append(rows, table.Row{
				p.Name, p.Schedule, p.SubvolumePath, ret, active,
			})
		}
		if len(rows) == 0 {
			rows = append(rows, table.Row{"—", "Sin políticas", "", "", ""})
		}
		return policiesDoneMsg{rows}
	}
}

// ── Table styles ──────────────────────────────────────────

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#2a2a4a")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("#f5c800"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#f5c800")).
		Background(lipgloss.Color("#1a1a35")).
		Bold(true)
	return s
}

// ── main ──────────────────────────────────────────────────

func main() {
	// Modo no-TUI: si hay args, usar CLI clásico
	if len(os.Args) > 1 {
		runCLI(os.Args[1:])
		return
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// CLI clásico para scripting
func runCLI(args []string) {
	switch args[0] {
	case "status":
		resp, err := http.Get(baseURL + "/health")
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		var data map[string]string
		json.NewDecoder(resp.Body).Decode(&data)
		fmt.Printf("status: %s  version: %s\n", data["status"], data["version"])

	case "version":
		fmt.Println("snapah pow CLI — v0.6.0")

	default:
		fmt.Println("Uso: snapah [status|version]")
		fmt.Println("Sin argumentos: abre la TUI interactiva")
	}
}

// Evitar warning de import no usado
var _ = strings.Contains
