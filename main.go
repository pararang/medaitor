package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/pat"
	"github.com/gorilla/websocket"
	"github.com/pararang/medaitor/db"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Message struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Content  string `json:"content"`
	Token    string `json:"token"`
}

var clients = make(map[*websocket.Conn]string)

func main() {
	db.Initialize()
	defer db.Close()

	router := pat.New()

	router.Handle("/", http.FileServer(http.Dir("./static")))
	router.Post("/register", handleRegister)
	router.Post("/login", handleLogin)
	router.Get("/messages", handleMessages)
	router.Handle("/ws", http.HandlerFunc(handleWebSocket))

	http.Handle("/", router)
	
	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	if err := db.RegisterUser(username, password); err != nil {
		handleDBError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	token, err := db.LoginUser(username, password)
	if err != nil {
		handleDBError(w, err)
		return
	}

	w.Write([]byte(token))
}

func handleMessages(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	userID, err := db.GetUserIDByToken(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	messages, err := db.GetMessageHistory(userID)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(messages)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
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

	userID, username, err := db.ValidateSession(auth.Token)
	if err != nil {
		ws.WriteJSON(Message{Type: "auth_failed"})
		return
	}

	clients[ws] = auth.Token
	defer delete(clients, ws)

	ws.WriteJSON(Message{Type: "auth_success", Username: username})

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
	for client := range clients {
		if err := client.WriteJSON(msg); err != nil {
			client.Close()
			delete(clients, client)
		}
	}
}

func handleDBError(w http.ResponseWriter, err error) {
	switch err.Error() {
	case "username exists":
		http.Error(w, "Username exists", http.StatusConflict)
	case "invalid credentials":
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
	default:
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}