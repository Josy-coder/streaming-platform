package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"time"

	"github.com/o1egl/paseto"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // Never sent to client
	StreamKey string    `json:"-"` // Never sent to client
	IsLive    bool      `json:"isLive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type UserRegister struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserLogin struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type Service struct {
	db     *sql.DB
	paseto *paseto.V2
}

// NewService creates a new auth service
func NewService(db *sql.DB) *Service {
	return &Service{
		db:     db,
		paseto: paseto.NewV2(),
	}
}

func (s *Service) Register(w http.ResponseWriter, r *http.Request) {
	var user UserRegister
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if user.Username == "" || user.Email == "" || user.Password == "" {
		http.Error(w, "Username, email, and password are required", http.StatusBadRequest)
		return
	}

	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", user.Username).Scan(&exists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Username already taken", http.StatusConflict)
		return
	}

	err = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)", user.Email).Scan(&exists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "Email already registered", http.StatusConflict)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error creating account", http.StatusInternalServerError)
		return
	}

	streamKey := generateStreamKey(user.Username)

	now := time.Now()
	result, err := s.db.Exec(
		"INSERT INTO users (username, email, password, stream_key, is_live, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		user.Username, user.Email, string(hashedPassword), streamKey, false, now, now,
	)
	if err != nil {
		http.Error(w, "Error creating account", http.StatusInternalServerError)
		return
	}

	userId, _ := result.LastInsertId()

	newUser := User{
		ID:        userId,
		Username:  user.Username,
		Email:     user.Email,
		IsLive:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	token, err := s.generateToken(userId, user.Username)
	if err != nil {
		http.Error(w, "Error generating auth token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User:  newUser,
	})
}

func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	var credentials UserLogin
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if credentials.Username == "" || credentials.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	var user User
	var hashedPassword string
	err := s.db.QueryRow(
		"SELECT id, username, email, password, is_live, created_at, updated_at FROM users WHERE username = ?",
		credentials.Username,
	).Scan(&user.ID, &user.Username, &user.Email, &hashedPassword, &user.IsLive, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(credentials.Password))
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	token, err := s.generateToken(user.ID, user.Username)
	if err != nil {
		http.Error(w, "Error generating auth token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User:  user,
	})
}

func (s *Service) GetProfile(w http.ResponseWriter, r *http.Request) {

	userId := r.Context().Value("userId").(int64)

	var user User
	err := s.db.QueryRow(
		"SELECT id, username, email, is_live, created_at, updated_at FROM users WHERE id = ?",
		userId,
	).Scan(&user.ID, &user.Username, &user.Email, &user.IsLive, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		http.Error(w, "Error fetching user profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (s *Service) UpdateProfile(w http.ResponseWriter, r *http.Request) {

	userId := r.Context().Value("userId").(int64)

	var updateData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if updateData.Email != "" {
		var exists bool
		err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ? AND id <> ?)",
			updateData.Email, userId).Scan(&exists)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "Email already registered", http.StatusConflict)
			return
		}

		_, err = tx.Exec("UPDATE users SET email = ?, updated_at = ? WHERE id = ?",
			updateData.Email, time.Now(), userId)
		if err != nil {
			http.Error(w, "Error updating email", http.StatusInternalServerError)
			return
		}
	}

	if updateData.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(updateData.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Error updating password", http.StatusInternalServerError)
			return
		}

		_, err = tx.Exec("UPDATE users SET password = ?, updated_at = ? WHERE id = ?",
			string(hashedPassword), time.Now(), userId)
		if err != nil {
			http.Error(w, "Error updating password", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var user User
	err = s.db.QueryRow(
		"SELECT id, username, email, is_live, created_at, updated_at FROM users WHERE id = ?",
		userId,
	).Scan(&user.ID, &user.Username, &user.Email, &user.IsLive, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		http.Error(w, "Error fetching updated user profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (s *Service) generateToken(userId int64, username string) (string, error) {
	now := time.Now()
	exp := now.Add(24 * time.Hour)

	jsonToken := paseto.JSONToken{
		Audience:   "streaming-platform",
		Issuer:     "streaming-platform",
		Jti:        "unique-token-id",
		Subject:    username,
		IssuedAt:   now,
		Expiration: exp,
		NotBefore:  now,
	}

	jsonToken.Set("user_id", strconv.FormatInt(userId, 10))

	footer := "streaming-platform-auth"
	secretKey := []byte("thisisatestkeydonotuseittheywillhackyou")

	token, err := s.paseto.Encrypt(secretKey, jsonToken, footer)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Service) ValidateToken(token string) (int64, string, error) {
	var jsonToken paseto.JSONToken
	var footer string

	secretKey := []byte("thisisatestkeyimadeitupdonotuseittheywillhackyou")

	err := s.paseto.Decrypt(token, secretKey, &jsonToken, &footer)
	if err != nil {
		return 0, "", errors.New("invalid token")
	}

	if jsonToken.Expiration.Before(time.Now()) {
		return 0, "", errors.New("token expired")
	}

	userIdStr := jsonToken.Get("user_id")
	if userIdStr == "" {
		return 0, "", errors.New("invalid token: missing user_id")
	}

	userId, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		return 0, "", errors.New("invalid token: user_id not a valid number")
	}

	return userId, jsonToken.Subject, nil
}

func generateStreamKey(username string) string {
	return username + "-" + uuid.New().String()
}
