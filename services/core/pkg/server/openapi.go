package server

import "net/http"

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(openAPIJSON))
}

const openAPIJSON = `{
  "openapi": "3.1.0",
  "info": { "title": "Zero Core API", "version": "0.1.0" },
  "paths": {
    "/health": { "get": { "responses": { "200": { "description": "OK" } } } },
    "/events": { "get": { "responses": { "200": { "description": "Server-Sent Events" } } } },
    "/openapi.json": { "get": { "responses": { "200": { "description": "OpenAPI document" } } } },
    "/projects/ensure": { "post": { "responses": { "200": { "description": "Project ensured" } } } },
    "/sessions": { "get": { "responses": { "200": { "description": "List sessions" } } }, "post": { "responses": { "201": { "description": "Create session" } } } },
    "/sessions/{id}": { "get": { "responses": { "200": { "description": "Get session" }, "404": { "description": "Not found" } } }, "patch": { "responses": { "200": { "description": "Update session" } } }, "delete": { "responses": { "204": { "description": "Archive session" } } } },
    "/sessions/{id}/run": { "post": { "responses": { "202": { "description": "Run agent for session" } } } },
    "/sessions/{id}/messages": { "get": { "responses": { "200": { "description": "List messages" } } }, "post": { "responses": { "201": { "description": "Create message" } } } },
    "/sessions/{id}/messages/{messageId}": { "delete": { "responses": { "204": { "description": "Delete message" } } } },
    "/collab/rooms": { "post": { "responses": { "201": { "description": "Create collaboration room" } } } },
    "/collab/rooms/{roomId}/join": { "post": { "responses": { "200": { "description": "Join collaboration room" } } } },
    "/collab/rooms/{roomId}/participants": { "get": { "responses": { "200": { "description": "List participants" } } } },
    "/collab/rooms/{roomId}/queue": { "get": { "responses": { "200": { "description": "List prompt queue" } } }, "post": { "responses": { "201": { "description": "Submit prompt" } } } },
    "/collab/rooms/{roomId}/events": { "get": { "responses": { "200": { "description": "Room Server-Sent Events" } } } }
  }
}`
