package alerts

import (
	"log"
	"time"

	"github.com/johandavid77/btrfs-snapah-pow/internal/storage"
)

type Watchdog struct {
	db      *storage.DB
	alerts  *Manager
	ticker  *time.Ticker
	stop    chan struct{}
	// estado anterior de nodos para detectar cambios
	lastStatus map[string]string
	interval   time.Duration
}

func NewWatchdog(db *storage.DB, alertsMgr *Manager, interval time.Duration) *Watchdog {
	return &Watchdog{
		db:         db,
		alerts:     alertsMgr,
		ticker:     time.NewTicker(interval),
		stop:       make(chan struct{}),
		lastStatus: make(map[string]string),
		interval:   interval,
	}
}

func (w *Watchdog) Start() {
	log.Printf("🐕 Watchdog iniciado — revisando cada %s", w.interval.String())
	go func() {
		for {
			select {
			case <-w.ticker.C:
				w.check()
			case <-w.stop:
				w.ticker.Stop()
				log.Println("🐕 Watchdog detenido")
				return
			}
		}
	}()
}

func (w *Watchdog) Stop() {
	close(w.stop)
}

func (w *Watchdog) check() {
	nodes, err := w.db.ListNodes()
	if err != nil {
		log.Printf("watchdog: error listando nodos: %v", err)
		return
	}

	offlineThreshold := 2 * time.Minute

	for _, n := range nodes {
		isOffline := time.Since(n.LastSeen) > offlineThreshold
		prevStatus := w.lastStatus[n.ID]

		if isOffline && prevStatus != "offline" {
			// Nodo acaba de caerse
			w.lastStatus[n.ID] = "offline"
			w.alerts.Send(Alert{
				Level:   "critical",
				Title:   "Nodo offline: " + n.Hostname,
				Message: "El nodo " + n.Hostname + " (" + n.Address + ") no responde desde hace más de 2 minutos.",
				NodeID:  n.ID,
			})
			// Actualizar estado en DB
			w.db.UpdateNodeStatus(n.ID, "offline")

		} else if !isOffline && prevStatus == "offline" {
			// Nodo volvió online
			w.lastStatus[n.ID] = "online"
			w.alerts.Send(Alert{
				Level:   "info",
				Title:   "Nodo recuperado: " + n.Hostname,
				Message: "El nodo " + n.Hostname + " está online nuevamente.",
				NodeID:  n.ID,
			})
			w.db.UpdateNodeStatus(n.ID, "online")

		} else if !isOffline && prevStatus == "" {
			w.lastStatus[n.ID] = "online"
		}
	}
}

// NotifySnapshotFailed alerta cuando un snapshot automático falla
func (w *Watchdog) NotifySnapshotFailed(nodeID, policy, err string) {
	w.alerts.Send(Alert{
		Level:   "error",
		Title:   "Snapshot fallido: " + policy,
		Message: "La política '" + policy + "' falló al crear snapshot. Error: " + err,
		NodeID:  nodeID,
	})
}

// NotifyReplicationFailed alerta cuando una replicación falla
func (w *Watchdog) NotifyReplicationFailed(nodeID, snapPath, err string) {
	w.alerts.Send(Alert{
		Level:   "error",
		Title:   "Replicación fallida",
		Message: "No se pudo replicar " + snapPath + ". Error: " + err,
		NodeID:  nodeID,
	})
}
