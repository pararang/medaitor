package handler

import (
	"net/http"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Content  string `json:"content"`
	Token    string `json:"token"`
	IsSelf   bool   `json:"is_self"`
}

type Identity struct {
	Username string
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	clients = make(map[*websocket.Conn]Identity)
)

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
