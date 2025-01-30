package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)

type Message struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func main() {
	// Add this line to serve static files
	http.Handle("/", http.FileServer(http.Dir("./static")))

	http.HandleFunc("/ws", handleConnections)
	go handleMessages()

	log.Println("Server starting on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Server error: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade initial GET request to WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("WebSocket upgrade error:", err)
	}
	defer func() {
		ws.Close()
		delete(clients, ws)
	}()

	clients[ws] = true // Add client to map

	// Continuous message reading
	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}
		broadcast <- msg
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("Write error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
