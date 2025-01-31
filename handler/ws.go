package handler

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pararang/medaitor/db"
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

	log.Println("auth", auth)

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
			if err := db.StoreMessage(userID, msg.Content); err != nil {
				log.Println("Message save error:", err)
			}
			broadcastMessage(Message{
				Type:     "message",
				Username: username,
				Content:  msg.Content,
			})
		}
	}
}

func broadcastMessage(msg Message) {
	clients.Range(func(key, value interface{}) bool {
		ws := key.(*wsClientConnection)
		identity := value.(Identity)
		msg.IsSelf = identity.Username == msg.Username
		if err := ws.WriteJSON(msg); err != nil {
			ws.Close()
			clients.Delete(ws)
		}
		return true
	})
}
