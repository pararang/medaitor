package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pararang/medaitor/db"
)

func Register(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	if err := db.RegisterUser(username, password); err != nil {
		handleDBError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func Login(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	token, err := db.LoginUser(username, password)
	if err != nil {
		handleDBError(w, err)
		return
	}

	w.Write([]byte(token))
}

func GetMessageHistories(w http.ResponseWriter, r *http.Request) {
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
