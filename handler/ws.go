package handler

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pararang/medaitor/db"
	"golang.org/x/sync/errgroup"
)

// wsClientConnection wraps websocket connection with a mutex for thread-safe writes
type wsClientConnection struct {
	conn     *websocket.Conn
	writeMux sync.Mutex
}

func (c *wsClientConnection) WriteJSON(v interface{}) error {
	c.writeMux.Lock()
	defer c.writeMux.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *wsClientConnection) Close() error {
	c.writeMux.Lock()
	defer c.writeMux.Unlock()
	return c.conn.Close()
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	clients = sync.Map{}
)

func WebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	ws := &wsClientConnection{conn: conn}
	defer ws.Close()

	var auth Message
	if err := conn.ReadJSON(&auth); err != nil || auth.Type != "auth" {
		ws.WriteJSON(Message{Type: "auth_failed"})
		return
	}

	userID, username, err := db.ValidateSession(auth.Token)
	if err != nil {
		ws.WriteJSON(Message{Type: "auth_failed"})
		return
	}

	clients.Store(ws, Identity{Username: username})

	defer func() {
		clients.Delete(ws)
		broadcastMessage(Message{
			Type:     "user_leave",
			Username: username,
		})
	}()

	ws.WriteJSON(Message{Type: "auth_success", Username: username})
	broadcastMessage(Message{
		Type:     "user_join",
		Username: username,
	})

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		if msg.Type == "message" && msg.Content != "" {
			var wg errgroup.Group

			wg.Go(func() error {
				errSm := db.StoreMessage(userID, msg.Content)
				if errSm != nil {
					return fmt.Errorf("failed to store message: %w", errSm)
				}

				return nil
			})

			wg.Go(func() error {
				broadcastMessage(Message{
					Type:     "message",
					Username: username,
					Content:  msg.Content,
				})

				return nil
			})

			if err := wg.Wait(); err != nil {
				log.Printf("failed processing message from %s: %v\n", username, err)
			}
		}
	}
}

func broadcastMessage(msg Message) {
	clients.Range(func(key, value interface{}) bool {
		ws := key.(*wsClientConnection)
		identity := value.(Identity)
		msg.IsSelf = identity.Username == msg.Username
		if err := ws.WriteJSON(msg); err != nil {
			log.Printf("failed broadcast message to %s: %v\n", identity.Username, err)
			ws.Close()
			clients.Delete(ws)
		}
		return true
	})
}
