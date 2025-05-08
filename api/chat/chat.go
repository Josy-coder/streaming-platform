package chat

import (
	"database/sql"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

type Message struct {
	ID        string    `json:"id,omitempty"`
	StreamID  string    `json:"streamId"`
	UserID    int64     `json:"userId"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Type      string    `json:"type"` // "text", "emoji", "system"
	CreatedAt time.Time `json:"createdAt"`
}
type Client struct {
	StreamID string
	UserID   int64
	Username string
	Conn     *websocket.Conn
	Send     chan []byte
}
type StreamHub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message
}
type Service struct {
	db           *sql.DB
	upgrader     websocket.Upgrader
	streams      map[string]*StreamHub
	streamsMutex sync.RWMutex
}

func NewService(db *sql.DB) *Service {
	return &Service{
		db: db,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all connections in dev
			},
		},
		streams: make(map[string]*StreamHub),
	}
}
func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamId")
	if streamID == "" {
		http.Error(w, "Stream ID is required", http.StatusBadRequest)
		return
	}
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM streams WHERE id = ?)", streamID).Scan(&exists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}
	userID, ok := r.Context().Value("userId").(int64)
	if !ok {
		userID = 0 // Anonymous user
	}
	username := "Anonymous"
	if userID > 0 {
		err := s.db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err != nil {
			log.Printf("Error fetching username: %v", err)
		}
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}
	hub := s.getOrCreateStreamHub(streamID)
	client := &Client{
		StreamID: streamID,
		UserID:   userID,
		Username: username,
		Conn:     conn,
		Send:     make(chan []byte, 256),
	}
	hub.register <- client
	if userID > 0 {
		hub.broadcast <- Message{
			StreamID:  streamID,
			UserID:    0, // System message
			Username:  "System",
			Content:   username + " has joined the chat",
			Type:      "system",
			CreatedAt: time.Now(),
		}
	}
	go s.writeMessages(client)
	go s.readMessages(hub, client)
}
func (s *Service) getOrCreateStreamHub(streamID string) *StreamHub {
	s.streamsMutex.RLock()
	hub, exists := s.streams[streamID]
	s.streamsMutex.RUnlock()
	if !exists {
		hub = &StreamHub{
			clients:    make(map[*Client]bool),
			register:   make(chan *Client),
			unregister: make(chan *Client),
			broadcast:  make(chan Message),
		}
		s.streamsMutex.Lock()
		s.streams[streamID] = hub
		s.streamsMutex.Unlock()
		go s.runStreamHub(hub)
	}
	return hub
}
func (s *Service) runStreamHub(hub *StreamHub) {
	for {
		select {
		case client := <-hub.register:
			hub.clients[client] = true
		case client := <-hub.unregister:
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				close(client.Send)
				if client.UserID > 0 {
					hub.broadcast <- Message{
						StreamID:  client.StreamID,
						UserID:    0, // System message
						Username:  "System",
						Content:   client.Username + " has left the chat",
						Type:      "system",
						CreatedAt: time.Now(),
					}
				}
			}
		case message := <-hub.broadcast:
			if message.Type == "text" || message.Type == "emoji" {
				_, err := s.db.Exec(`
					INSERT INTO messages (stream_id, user_id, username, content, type, created_at)
					VALUES (?, ?, ?, ?, ?, ?)
				`, message.StreamID, message.UserID, message.Username, message.Content, message.Type, message.CreatedAt)
				if err != nil {
					log.Printf("Error storing message: %v", err)
				}
			}
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error encoding message: %v", err)
				continue
			}
			for client := range hub.clients {
				select {
				case client.Send <- data:
				default:
					close(client.Send)
					delete(hub.clients, client)
				}
			}
		}
	}
}
func (s *Service) readMessages(hub *StreamHub, client *Client) {
	defer func() {
		hub.unregister <- client
		client.Conn.Close()
	}()
	client.Conn.SetReadLimit(4096) // 4KB limit
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, data, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			break
		}
		var message Message
		if err := json.Unmarshal(data, &message); err != nil {
			log.Printf("Error decoding message: %v", err)
			continue
		}
		if message.Content == "" {
			continue
		}
		message.StreamID = client.StreamID
		message.UserID = client.UserID
		message.Username = client.Username
		message.CreatedAt = time.Now()
		if message.Type == "" {
			message.Type = "text"
		}
		if message.Type == "emoji" {
			hub.broadcast <- message
			continue
		}
		if len(message.Content) > 500 {
			message.Content = message.Content[:500]
		}
		hub.broadcast <- message
	}
}
func (s *Service) writeMessages(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := client.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.Send)
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
func (s *Service) GetChatHistory(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamId")
	if streamID == "" {
		http.Error(w, "Stream ID is required", http.StatusBadRequest)
		return
	}
	rows, err := s.db.Query(`
		SELECT id, stream_id, user_id, username, content, type, created_at
		FROM messages
		WHERE stream_id = ?
		ORDER BY created_at DESC
		LIMIT 100
	`, streamID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(
			&message.ID, &message.StreamID, &message.UserID, &message.Username,
			&message.Content, &message.Type, &message.CreatedAt,
		); err != nil {
			http.Error(w, "Error scanning message", http.StatusInternalServerError)
			return
		}
		messages = append(messages, message)
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}
