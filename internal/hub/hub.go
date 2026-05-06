package hub

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Time    time.Time   `json:"time"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

type client struct {
	conn *websocket.Conn
	send chan Event
}

func New() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

// Broadcast envía un evento a todos los clientes conectados
func (h *Hub) Broadcast(eventType string, payload interface{}) {
	evt := Event{Type: eventType, Payload: payload, Time: time.Now()}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- evt:
		default:
			// cliente lento, saltamos
		}
	}
}

// ServeWS maneja la conexión WebSocket
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // permitir cualquier origen en dev
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}

	c := &client{conn: conn, send: make(chan Event, 32)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	log.Printf("ws: cliente conectado (%d total)", len(h.clients))

	// ping keepalive
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.Ping(ctx); err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// enviar eventos
	go func() {
		for {
			select {
			case evt := <-c.send:
				if err := wsjson.Write(ctx, conn, evt); err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// esperar cierre
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			break
		}
	}

	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	conn.Close(websocket.StatusNormalClosure, "")
	log.Printf("ws: cliente desconectado (%d total)", len(h.clients))
}

// PingPayload para el endpoint de health ws
type PingPayload struct {
	Message string `json:"message"`
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// MarshalEvent helper
func MarshalEvent(eventType string, payload interface{}) []byte {
	evt := Event{Type: eventType, Payload: payload, Time: time.Now()}
	b, _ := json.Marshal(evt)
	return b
}
