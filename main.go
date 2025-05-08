package main

import (
	_ "embed"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/josy-coder/streaming-platfrom/api/auth"
	"github.com/josy-coder/streaming-platfrom/api/chat"
	"github.com/josy-coder/streaming-platfrom/api/config"
	"github.com/josy-coder/streaming-platfrom/api/db"
	"github.com/josy-coder/streaming-platfrom/api/middleware"
	"github.com/josy-coder/streaming-platfrom/api/stream"
	"log"
	"net/http"
	_ "os"
	"time"
)

func main() {
	cfg := config.Load()
	database, err := db.InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()
	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	authService := auth.NewService(database)
	streamService := stream.NewService(database)
	chatService := chat.NewService(database)
	r.Group(func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Streaming Platform API"))
		})
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		})
		r.Post("/api/auth/register", authService.Register)
		r.Post("/api/auth/login", authService.Login)
		r.Get("/api/streams", streamService.ListStreams)
		r.Get("/api/streams/{streamId}", streamService.GetStream)
	})
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(authService))
		r.Get("/api/user", authService.GetProfile)
		r.Put("/api/user", authService.UpdateProfile)
		r.Post("/api/streams", streamService.CreateStream)
		r.Put("/api/streams/{streamId}", streamService.UpdateStream)
		r.Delete("/api/streams/{streamId}", streamService.DeleteStream)
		r.Get("/api/streams/key", streamService.GetStreamKey)
		r.Post("/api/streams/key/regenerate", streamService.RegenerateStreamKey)
	})
	r.Get("/ws/chat/{streamId}", chatService.HandleWebSocket)
	port := cfg.Port
	if port == "" {
		port = "3000"
	}
	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
