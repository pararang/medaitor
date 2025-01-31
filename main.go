package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/pat"
	"github.com/pararang/medaitor/db"
	"github.com/pararang/medaitor/handler"
)

var (
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
	router.Post("/register", handler.Register)
	router.Post("/login", handler.Login)
	router.Get("/messages", handler.GetMessageHistories)
	router.Handle("/", http.FileServer(http.Dir("./static")))
	router.Handle("/ws", http.HandlerFunc(handler.WebSocket))

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

		err := db.Close()
		if err != nil {
			log.Fatalf("Could not close database %+v\n", err)
		} else {
			log.Println("Database connection closed")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server %+v\n", err)
		}

		close(done)
	}()

	log.Println("Server is running on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on :8080 %+v\n", err)
	}

	<-done
	log.Println("Server stopped")
}
