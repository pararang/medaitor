package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func Initialize() error {
	var err error
	db, err = sql.Open("sqlite", "file:chat.db?_fk=true")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
	`)
	if err != nil {
		return fmt.Errorf("Database initialization failed: %w", err)
	}

	return nil
}

func Close() {
	db.Close()
}

func RegisterUser(username, password string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", username, hashed)
	if err != nil {
		if err.Error() == "UNIQUE constraint failed: users.username" {
			return errors.New("username exists")
		}
		return err
	}
	return nil
}

func LoginUser(username, password string) (string, error) {
	var user User
	err := db.QueryRow("SELECT id, password FROM users WHERE username = ?", username).
		Scan(&user.ID, &user.Password)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	token, _ := bcrypt.GenerateFromPassword([]byte(username+time.Now().String()), 10)
	tokenStr := string(token)

	_, err = db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		tokenStr, user.ID, time.Now().Add(24*time.Hour))
	if err != nil {
		return "", errors.New("login failed")
	}

	return tokenStr, nil
}

func ValidateSession(token string) (int, string, error) {
	var userID int
	var username string
	var expires time.Time

	err := db.QueryRow(`
		SELECT s.user_id, u.username, s.expires_at 
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > CURRENT_TIMESTAMP
	`, token).Scan(&userID, &username, &expires)

	if err != nil {
		return 0, "", errors.New("invalid token")
	}
	return userID, username, nil
}

func StoreMessage(userID int, content string) error {
	_, err := db.Exec("INSERT INTO messages (user_id, content) VALUES (?, ?)", userID, content)
	return err
}

func GetMessageHistory(userID int) ([]Message, error) {
	rows, err := db.Query(`
		SELECT u.username, m.content, m.created_at 
		FROM messages m
		JOIN users u ON m.user_id = u.id
		ORDER BY m.created_at DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		rows.Scan(&msg.Username, &msg.Content, &msg.CreatedAt)
		messages = append(messages, msg)
	}
	return messages, nil
}

func GetUserIDByToken(token string) (int, error) {
	var userID int
	err := db.QueryRow(`
		SELECT user_id 
		FROM sessions 
		WHERE token = ? AND expires_at > CURRENT_TIMESTAMP
	`, token).Scan(&userID)
	return userID, err
}