package handler

import (
	"log"
	"net/http"

	"github.com/pararang/medaitor/db"
)

func WebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	var auth Message
	if err := ws.ReadJSON(&auth); err != nil || auth.Type != "auth" {
		ws.WriteJSON(Message{Type: "auth_failed"})
		return
	}

	log.Println("auth", auth)

	userID, username, err := db.ValidateSession(auth.Token)
	if err != nil {
		ws.WriteJSON(Message{Type: "auth_failed"})
		return
	}

	clients[ws] = Identity{
		Username: username,
	}

	defer delete(clients, ws)

	ws.WriteJSON(Message{Type: "auth_success", Username: username})
	broadcastMessage(Message{
		Type:     "user_join",
		Username: username,
	})

	// Handle client disconnection
	defer func() {
		broadcastMessage(Message{
			Type:     "user_leave",
			Username: username,
		})
	}()

	for {
		var msg Message
		if err := ws.ReadJSON(&msg); err != nil {
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
	for client, identity := range clients {
		msg.IsSelf = identity.Username == msg.Username
		if err := client.WriteJSON(msg); err != nil {
			client.Close()
			delete(clients, client)
		}
	}
}
