package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"gemini-antiblock/logger"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
	Version   string    `json:"version,omitempty"`
}

// HealthHandler handles health check requests
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	logger.LogDebug("Health check endpoint accessed")

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Service:   "gemini-antiblock-proxy",
		Version:   "0.2.0",
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.LogError("Failed to encode health response:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.LogDebug("Health check response sent successfully")
}
