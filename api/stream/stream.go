package stream

import (
	"database/sql"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"log"
	"net/http"
	"os"
	"path/filepath"
	_ "strings"
	"time"
)

type Stream struct {
	ID           string    `json:"id"`
	UserID       int64     `json:"userId"`
	Username     string    `json:"username"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	IsLive       bool      `json:"isLive"`
	ViewerCount  int       `json:"viewerCount"`
	StartedAt    time.Time `json:"startedAt"`
	EndedAt      time.Time `json:"endedAt,omitempty"`
	ThumbnailURL string    `json:"thumbnailUrl"`
}
type StreamUpdate struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
type StreamKeyResponse struct {
	StreamKey string `json:"streamKey"`
}
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{
		db: db,
	}
}
func (s *Service) ListStreams(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = s.db.Query(`
			SELECT s.id, s.user_id, u.username, s.title, s.description, 
				   s.category, s.is_live, s.viewer_count, s.started_at, s.ended_at, s.thumbnail_url
			FROM streams s
			JOIN users u ON s.user_id = u.id
			WHERE s.is_live = 1 AND s.category = ?
			ORDER BY s.viewer_count DESC
		`, category)
	} else {
		rows, err = s.db.Query(`
			SELECT s.id, s.user_id, u.username, s.title, s.description, 
				   s.category, s.is_live, s.viewer_count, s.started_at, s.ended_at, s.thumbnail_url
			FROM streams s
			JOIN users u ON s.user_id = u.id
			WHERE s.is_live = 1
			ORDER BY s.viewer_count DESC
		`)
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var streams []Stream
	for rows.Next() {
		var stream Stream
		var endedAt sql.NullTime
		err := rows.Scan(
			&stream.ID, &stream.UserID, &stream.Username, &stream.Title,
			&stream.Description, &stream.Category, &stream.IsLive,
			&stream.ViewerCount, &stream.StartedAt, &endedAt, &stream.ThumbnailURL,
		)
		if err != nil {
			http.Error(w, "Error scanning stream data", http.StatusInternalServerError)
			return
		}
		if endedAt.Valid {
			stream.EndedAt = endedAt.Time
		}
		streams = append(streams, stream)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(streams)
}
func (s *Service) GetStream(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamId")
	if streamID == "" {
		http.Error(w, "Stream ID is required", http.StatusBadRequest)
		return
	}
	var stream Stream
	var endedAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT s.id, s.user_id, u.username, s.title, s.description, 
			   s.category, s.is_live, s.viewer_count, s.started_at, s.ended_at, s.thumbnail_url
		FROM streams s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = ?
	`, streamID).Scan(
		&stream.ID, &stream.UserID, &stream.Username, &stream.Title,
		&stream.Description, &stream.Category, &stream.IsLive,
		&stream.ViewerCount, &stream.StartedAt, &endedAt, &stream.ThumbnailURL,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Stream not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}
	if endedAt.Valid {
		stream.EndedAt = endedAt.Time
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stream)
	go func() {
		_, err := s.db.Exec("UPDATE streams SET viewer_count = viewer_count + 1 WHERE id = ?", streamID)
		if err != nil {
			log.Printf("Error updating viewer count: %v", err)
		}
	}()
}
func (s *Service) CreateStream(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userId").(int64)
	var streamData StreamUpdate
	if err := json.NewDecoder(r.Body).Decode(&streamData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if streamData.Title == "" {
		http.Error(w, "Stream title is required", http.StatusBadRequest)
		return
	}
	var existingStream string
	err := s.db.QueryRow("SELECT id FROM streams WHERE user_id = ? AND is_live = 1", userID).Scan(&existingStream)
	if err == nil {
		http.Error(w, "You already have an active stream", http.StatusConflict)
		return
	} else if err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	streamID := uuid.New().String()
	thumbnailURL := "/assets/default-thumbnail.jpg"
	now := time.Now()
	_, err = s.db.Exec(`
		INSERT INTO streams (id, user_id, title, description, category, is_live, viewer_count, started_at, thumbnail_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, streamID, userID, streamData.Title, streamData.Description, streamData.Category, true, 0, now, thumbnailURL)
	if err != nil {
		http.Error(w, "Error creating stream", http.StatusInternalServerError)
		return
	}
	_, err = s.db.Exec("UPDATE users SET is_live = 1 WHERE id = ?", userID)
	if err != nil {
		log.Printf("Error updating user's live status: %v", err)
	}
	var username string
	err = s.db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
	if err != nil {
		http.Error(w, "Error fetching user data", http.StatusInternalServerError)
		return
	}
	stream := Stream{
		ID:           streamID,
		UserID:       userID,
		Username:     username,
		Title:        streamData.Title,
		Description:  streamData.Description,
		Category:     streamData.Category,
		IsLive:       true,
		ViewerCount:  0,
		StartedAt:    now,
		ThumbnailURL: thumbnailURL,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stream)
}
func (s *Service) UpdateStream(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userId").(int64)
	streamID := chi.URLParam(r, "streamId")
	if streamID == "" {
		http.Error(w, "Stream ID is required", http.StatusBadRequest)
		return
	}
	var streamData StreamUpdate
	if err := json.NewDecoder(r.Body).Decode(&streamData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	var streamExists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM streams WHERE id = ? AND user_id = ?)",
		streamID, userID).Scan(&streamExists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if !streamExists {
		http.Error(w, "Stream not found or not owned by you", http.StatusNotFound)
		return
	}
	_, err = s.db.Exec(`
		UPDATE streams 
		SET title = ?, description = ?, category = ?
		WHERE id = ? AND user_id = ?
	`, streamData.Title, streamData.Description, streamData.Category, streamID, userID)
	if err != nil {
		http.Error(w, "Error updating stream", http.StatusInternalServerError)
		return
	}
	var stream Stream
	var endedAt sql.NullTime
	err = s.db.QueryRow(`
		SELECT s.id, s.user_id, u.username, s.title, s.description, 
			   s.category, s.is_live, s.viewer_count, s.started_at, s.ended_at, s.thumbnail_url
		FROM streams s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = ?
	`, streamID).Scan(
		&stream.ID, &stream.UserID, &stream.Username, &stream.Title,
		&stream.Description, &stream.Category, &stream.IsLive,
		&stream.ViewerCount, &stream.StartedAt, &endedAt, &stream.ThumbnailURL,
	)
	if err != nil {
		http.Error(w, "Error fetching updated stream", http.StatusInternalServerError)
		return
	}
	if endedAt.Valid {
		stream.EndedAt = endedAt.Time
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stream)
}
func (s *Service) DeleteStream(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userId").(int64)
	streamID := chi.URLParam(r, "streamId")
	if streamID == "" {
		http.Error(w, "Stream ID is required", http.StatusBadRequest)
		return
	}
	var streamExists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM streams WHERE id = ? AND user_id = ? AND is_live = 1)",
		streamID, userID).Scan(&streamExists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if !streamExists {
		http.Error(w, "Stream not found, not owned by you, or already ended", http.StatusNotFound)
		return
	}
	now := time.Now()
	_, err = s.db.Exec("UPDATE streams SET is_live = 0, ended_at = ? WHERE id = ?", now, streamID)
	if err != nil {
		http.Error(w, "Error ending stream", http.StatusInternalServerError)
		return
	}
	_, err = s.db.Exec("UPDATE users SET is_live = 0 WHERE id = ?", userID)
	if err != nil {
		log.Printf("Error updating user's live status: %v", err)
	}
	mediaDir := filepath.Join("media", "archive", streamID)
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		log.Printf("Error creating archive directory: %v", err)
	}
	go func() {
		srcDir := filepath.Join("media", "hls", streamID)
		files, err := os.ReadDir(srcDir)
		if err != nil {
			log.Printf("Error reading HLS directory: %v", err)
			return
		}
		for _, file := range files {
			srcPath := filepath.Join(srcDir, file.Name())
			dstPath := filepath.Join(mediaDir, file.Name())
			if err := os.Rename(srcPath, dstPath); err != nil {
				log.Printf("Error moving file %s: %v", file.Name(), err)
			}
		}
		if err := os.Remove(srcDir); err != nil {
			log.Printf("Error removing source directory: %v", err)
		}
	}()
	w.WriteHeader(http.StatusNoContent)
}
func (s *Service) GetStreamKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userId").(int64)
	var streamKey string
	err := s.db.QueryRow("SELECT stream_key FROM users WHERE id = ?", userID).Scan(&streamKey)
	if err != nil {
		http.Error(w, "Error fetching stream key", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StreamKeyResponse{
		StreamKey: streamKey,
	})
}
func (s *Service) RegenerateStreamKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userId").(int64)
	var username string
	err := s.db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
	if err != nil {
		http.Error(w, "Error fetching user data", http.StatusInternalServerError)
		return
	}
	newStreamKey := username + "-" + uuid.New().String()
	_, err = s.db.Exec("UPDATE users SET stream_key = ? WHERE id = ?", newStreamKey, userID)
	if err != nil {
		http.Error(w, "Error updating stream key", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StreamKeyResponse{
		StreamKey: newStreamKey,
	})
}
func (s *Service) OnStreamStart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	streamKey := r.FormValue("name")
	if streamKey == "" {
		http.Error(w, "Stream key is required", http.StatusBadRequest)
		return
	}
	var userID int64
	var username string
	err := s.db.QueryRow("SELECT id, username FROM users WHERE stream_key = ?", streamKey).Scan(&userID, &username)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid stream key", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}
	var existingStreamID string
	err = s.db.QueryRow("SELECT id FROM streams WHERE user_id = ? AND is_live = 1", userID).Scan(&existingStreamID)
	streamID := ""
	if err == nil {
		streamID = existingStreamID
	} else if err == sql.ErrNoRows {
		streamID = uuid.New().String()
		thumbnailURL := "/assets/default-thumbnail.jpg"
		now := time.Now()
		_, err = s.db.Exec(`
			INSERT INTO streams (id, user_id, title, description, category, is_live, viewer_count, started_at, thumbnail_url)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, streamID, userID, username+"'s Stream", "", "General", true, 0, now, thumbnailURL)
		if err != nil {
			http.Error(w, "Error creating stream", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	_, err = s.db.Exec("UPDATE users SET is_live = 1 WHERE id = ?", userID)
	if err != nil {
		log.Printf("Error updating user's live status: %v", err)
	}
	mediaDir := filepath.Join("media", "hls", streamID)
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		log.Printf("Error creating media directory: %v", err)
		http.Error(w, "Error creating media directory", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"streamId": streamID,
	})
}
func (s *Service) OnStreamEnd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	streamKey := r.FormValue("name")
	if streamKey == "" {
		http.Error(w, "Stream key is required", http.StatusBadRequest)
		return
	}
	var userID int64
	err := s.db.QueryRow("SELECT id FROM users WHERE stream_key = ?", streamKey).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid stream key", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}
	var streamID string
	err = s.db.QueryRow("SELECT id FROM streams WHERE user_id = ? AND is_live = 1", userID).Scan(&streamID)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNoContent)
			return
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
	}
	now := time.Now()
	_, err = s.db.Exec("UPDATE streams SET is_live = 0, ended_at = ? WHERE id = ?", now, streamID)
	if err != nil {
		http.Error(w, "Error ending stream", http.StatusInternalServerError)
		return
	}
	_, err = s.db.Exec("UPDATE users SET is_live = 0 WHERE id = ?", userID)
	if err != nil {
		log.Printf("Error updating user's live status: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
}
