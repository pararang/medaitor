package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
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
	IsSelf   bool   `json:"is_self"`
}

type Identity struct {
	Username string
}

var (
	clients = make(map[*websocket.Conn]Identity)
	healthy = int32(1)
)

func healthCheck(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&healthy) == 1 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func main() {
	err := db.Initialize()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	router := pat.New()
	router.Get("/health", healthCheck)
	router.Post("/register", handleRegister)
	router.Post("/login", handleLogin)
	router.Get("/messages", handleMessages)
	router.Handle("/", http.FileServer(http.Dir("./static")))
	router.Handle("/ws", http.HandlerFunc(handleWebSocket))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handlers.LoggingHandler(os.Stdout, handlers.CompressHandler(router)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down ....")
		atomic.StoreInt32(&healthy, 0)
		db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server %+v\n", err)
		}

		close(done)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on :8080 %+v\n", err)
	}

	<-done
	log.Println("Server stopped")
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
