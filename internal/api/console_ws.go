package api

import (
	"bufio"
	"log"
	"net/http"

	"github.com/Julakk/DockWings/internal/docker"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ConsoleWS upgrade koneksi ke WebSocket, stream log container secara live,
// dan terusin pesan masuk dari browser sebagai command console.
//
// Auth di sini beda dari endpoint lain: browser nggak bisa kirim header
// Authorization custom pas buka WebSocket, jadi token dikirim lewat query
// param ?token=..., berupa JWT short-lived dari Panel.
func (h *Handlers) ConsoleWS(authToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")

		claims, err := verifyWSToken(r.URL.Query().Get("token"), authToken)
		if err != nil || claims.ServerUUID != uuid {
			http.Error(w, `{"error":"token console nggak valid"}`, http.StatusUnauthorized)
			return
		}

		s, err := h.Manager.Get(uuid)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
			return
		}

		streamer, ok := h.Env.(docker.LogStreamer)
		if !ok {
			http.Error(w, `{"error":"environment ini belum support log streaming"}`, http.StatusNotImplemented)
			return
		}

		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("gagal upgrade websocket: %v", err)
			return
		}
		defer conn.Close()

		ctx := r.Context()
		logs, err := streamer.StreamLogs(ctx, s)
		if err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte("[error] gagal ambil log: "+err.Error()))
			return
		}
		defer logs.Close()

		done := make(chan struct{})

		go func() {
			defer close(done)
			scanner := bufio.NewScanner(logs)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				if err := conn.WriteMessage(websocket.TextMessage, scanner.Bytes()); err != nil {
					return
				}
			}
		}()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if len(msg) == 0 {
				continue
			}
			if err := h.Env.SendCommand(ctx, s, string(msg)); err != nil {
				_ = conn.WriteMessage(websocket.TextMessage, []byte("[error] "+err.Error()))
			}
		}

		<-done
	}
}
