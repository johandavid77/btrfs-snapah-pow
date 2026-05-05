package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	gomail "gopkg.in/gomail.v2"
)

type Config struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string
	WebhookURL   string
	EmailEnabled bool
	WebhookEnabled bool
}

type Alert struct {
	Level   string    // info | warning | error | critical
	Title   string
	Message string
	NodeID  string
	Time    time.Time
}

type Manager struct {
	cfg Config
}

func NewManager(cfg Config) *Manager {
	return &Manager{cfg: cfg}
}

func ConfigFromEnv() Config {
	port := 587
	return Config{
		SMTPHost:       getEnv("SMTP_HOST", "localhost"),
		SMTPPort:       port,
		SMTPUser:       getEnv("SMTP_USERNAME", ""),
		SMTPPass:       getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:       getEnv("SMTP_FROM", "snapah@localhost"),
		WebhookURL:     getEnv("ALERT_WEBHOOK_URL", ""),
		EmailEnabled:   getEnv("ALERT_EMAIL_TO", "") != "",
		WebhookEnabled: getEnv("ALERT_WEBHOOK_URL", "") != "",
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Send despacha la alerta por todos los canales configurados
func (m *Manager) Send(a Alert) {
	if a.Time.IsZero() {
		a.Time = time.Now()
	}
	log.Printf("🔔 ALERTA [%s] %s: %s", a.Level, a.Title, a.Message)

	if m.cfg.EmailEnabled {
		go func() {
			if err := m.sendEmail(a); err != nil {
				log.Printf("⚠️  Email alerta falló: %v", err)
			}
		}()
	}

	if m.cfg.WebhookEnabled {
		go func() {
			if err := m.sendWebhook(a); err != nil {
				log.Printf("⚠️  Webhook alerta falló: %v", err)
			}
		}()
	}
}

func (m *Manager) sendEmail(a Alert) error {
	to := os.Getenv("ALERT_EMAIL_TO")
	if to == "" {
		return nil
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.cfg.SMTPFrom)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", fmt.Sprintf("[snapah-pow][%s] %s", a.Level, a.Title))
	msg.SetBody("text/html", emailBody(a))

	d := gomail.NewDialer(m.cfg.SMTPHost, m.cfg.SMTPPort, m.cfg.SMTPUser, m.cfg.SMTPPass)
	if err := d.DialAndSend(msg); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	log.Printf("📧 Email enviado a %s", to)
	return nil
}

func (m *Manager) sendWebhook(a Alert) error {
	payload := map[string]interface{}{
		"level":   a.Level,
		"title":   a.Title,
		"message": a.Message,
		"node_id": a.NodeID,
		"time":    a.Time.Format(time.RFC3339),
		"source":  "snapah-pow",
	}

	// Slack-compatible format
	slackPayload := map[string]interface{}{
		"text": fmt.Sprintf("*[snapah-pow]* %s\n%s", a.Title, a.Message),
		"attachments": []map[string]interface{}{
			{
				"color":  levelColor(a.Level),
				"fields": []map[string]string{
					{"title": "Nivel", "value": a.Level, "short": "true"},
					{"title": "Nodo", "value": a.NodeID, "short": "true"},
					{"title": "Hora", "value": a.Time.Format("02/01 15:04:05"), "short": "true"},
				},
			},
		},
	}

	// Intentar como Slack primero, luego JSON genérico
	body, _ := json.Marshal(slackPayload)
	resp, err := http.Post(m.cfg.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		// Fallback a JSON genérico
		body, _ = json.Marshal(payload)
		resp, err = http.Post(m.cfg.WebhookURL, "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("webhook: %w", err)
		}
	}
	defer resp.Body.Close()
	log.Printf("🔗 Webhook enviado: %d", resp.StatusCode)
	return nil
}

func levelColor(level string) string {
	switch level {
	case "critical":
		return "#e8360a"
	case "error":
		return "#e8360a"
	case "warning":
		return "#f5c800"
	default:
		return "#22a64a"
	}
}

func emailBody(a Alert) string {
	color := levelColor(a.Level)
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family:monospace;background:#0d0d1a;color:#e8e8f0;padding:32px;">
  <div style="max-width:560px;margin:0 auto;">
    <h2 style="color:%s;margin-bottom:8px;">%s</h2>
    <p style="color:#7878a0;font-size:12px;margin-bottom:24px;">snapah pow · %s</p>
    <div style="background:#1a1a35;border:1px solid #2a2a4a;border-radius:8px;padding:20px;">
      <p style="margin:0 0 12px 0;">%s</p>
      <hr style="border:none;border-top:1px solid #2a2a4a;margin:16px 0;">
      <p style="color:#7878a0;font-size:12px;margin:0;">
        Nivel: <b style="color:%s">%s</b> &nbsp;·&nbsp;
        Nodo: <code>%s</code> &nbsp;·&nbsp;
        Hora: %s
      </p>
    </div>
    <div style="margin-top:24px;display:flex;gap:4px;">
      <div style="height:4px;flex:1;background:#e8360a;border-radius:2px;"></div>
      <div style="height:4px;flex:1;background:#f5c800;border-radius:2px;"></div>
      <div style="height:4px;flex:1;background:#22a64a;border-radius:2px;"></div>
    </div>
  </div>
</body>
</html>`, color, a.Title, a.Time.Format("02/01/2006 15:04:05"),
		a.Message, color, a.Level, a.NodeID,
		a.Time.Format("02/01/2006 15:04:05"))
}
