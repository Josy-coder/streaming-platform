# StreamLive Platform

A lightweight, self-hosted streaming platform built with Go, SQLite, and Alpine.js.

## Features

- Live streaming with RTMP/HLS
- User authentication
- Real-time chat with WebSockets
- Stream categories and browsing
- Streamer dashboard

## Tech Stack

- Backend: Go (Chi router)
- Database: SQLite
- Frontend: Alpine.js & Tailwind CSS
- Streaming: NGINX RTMP Module & FFmpeg
- Chat: WebSockets with Valkey (Redis fork)
- Containerization: Docker

## Setup

1. Install Docker and Docker Compose
2. Clone this repository
3. Run `docker-compose up -d` in the docker directory
4. Access the platform at http://localhost:8080


## License

MIT