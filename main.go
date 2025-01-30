package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]string)
var broadcast = make(chan Message)

type Message struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Content  string `json:"content"`
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/ws", handleConnections)
	go handleMessages()
	
	log.Println("Server running on :8080")
	http.ListenAndServe(":8080", nil)
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// Auth handshake
	ws.WriteJSON(Message{Type: "auth_request"})
	
	var authMsg Message
	if err := ws.ReadJSON(&authMsg); err != nil || authMsg.Type != "auth" || authMsg.Username == "" {
		ws.WriteJSON(Message{Type: "auth_failed"})
		return
	}

	clients[ws] = authMsg.Username
	ws.WriteJSON(Message{Type: "auth_success"})

	// Message loop
	for {
		var msg Message
		if err := ws.ReadJSON(&msg); err != nil {
			delete(clients, ws)
			break
		}

		if msg.Type == "message" {
			msg.Username = clients[ws]
			broadcast <- msg
		}
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}