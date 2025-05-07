package config

import 
import (
	"os"
	"path/filepath"
C
)
	Port         string
	Port string
	MediaPath    string
	SecretKey    string
	RtmpPort     string
	RtmpPort string
n
}
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/streaming.db"
	}
	mediaPath := os.Getenv("MEDIA_PATH")
	if mediaPath == "" {
		mediaPath = "media"
	}
	ensureDir(filepath.Join(mediaPath, "hls"))
	ensureDir(filepath.Join(mediaPath, "archive"))
	ensureDir(filepath.Join(mediaPath, "thumbnails"))
	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		secretKey = "supersecretkeythatyouneedtochange" // In production, always set this!
	}
	rtmpPort := os.Getenv("RTMP_PORT")
	if rtmpPort == "" {
		rtmpPort = "1935"
	}
	return &Config{
		Port:         port,
		DatabasePath: dbPath,
		MediaPath:    mediaPath,
		SecretKey:    secretKey,
		RtmpPort:     rtmpPort,
	}
}
func ensureDir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(path, 0755)
	}
}
