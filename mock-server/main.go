package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// TestCase represents different test scenarios
type TestCase int

const (
	// TestCaseNoEndMarker outputs thinking parts but doesn't return [done] at the end
	TestCaseNoEndMarker TestCase = 1
	// TestCaseSplitEndMarker splits the [done] mark at the end
	TestCaseSplitEndMarker TestCase = 2
	// TestCaseEmptyResponse returns an empty response
	TestCaseEmptyResponse TestCase = 3
)

// MockServer handles the mock API requests
type MockServer struct {
	baseDelay time.Duration
}

// NewMockServer creates a new mock server instance
func NewMockServer() *MockServer {
	return &MockServer{
		baseDelay: 50 * time.Millisecond, // Base delay between chunks
	}
}

// extractTestCaseFromPath extracts test case number from URL path
func (ms *MockServer) extractTestCaseFromPath(path string) int {
	// Look for patterns like /type-1, /type-2, /type-3
	if strings.Contains(path, "/type-1") {
		return 1
	}
	if strings.Contains(path, "/type-2") {
		return 2
	}
	if strings.Contains(path, "/type-3") {
		return 3
	}

	// Default to test case 1 if no specific type found
	return 1
}

// randomDelay adds a random delay to simulate real API behavior
func (ms *MockServer) randomDelay() {
	// Random delay between 50ms to 200ms
	delay := ms.baseDelay + time.Duration(rand.Intn(150))*time.Millisecond
	time.Sleep(delay)
}

// writeSSEData writes a data line in SSE format
func (ms *MockServer) writeSSEData(w http.ResponseWriter, data interface{}) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	ms.randomDelay()
}

// handleTestCase1 - No end marker, with thinking parts
func (ms *MockServer) handleTestCase1(w http.ResponseWriter) {
	log.Println("Handling test case 1: No end marker with thinking parts")

	// Thinking part 1
	thinkingData1 := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text":    "Let me think about this question...",
							"thought": true,
						},
					},
				},
			},
		},
	}
	ms.writeSSEData(w, thinkingData1)

	// Thinking part 2
	thinkingData2 := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text":    "I need to consider multiple aspects of this problem.",
							"thought": true,
						},
					},
				},
			},
		},
	}
	ms.writeSSEData(w, thinkingData2)

	// Regular content chunks
	contentChunks := []string{
		"Based on your question, I can provide the following response:\n\n",
		"This is a mock response that simulates a streaming API. ",
		"The response is being delivered in chunks with random delays. ",
		"This particular test case (Case 1) includes thinking parts but ",
		"deliberately does not include a [done] marker at the end. ",
		"This helps test scenarios where the stream might be interrupted ",
		"before completion.",
	}

	for i, chunk := range contentChunks {
		contentData := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"text": chunk,
							},
						},
					},
				},
			},
		}

		// Add finishReason to the last chunk
		if i == len(contentChunks)-1 {
			contentData["candidates"].([]map[string]interface{})[0]["finishReason"] = "STOP"
		}

		ms.writeSSEData(w, contentData)
	}

	// Note: Deliberately not sending [done] marker
	log.Println("Test case 1 completed without [done] marker")
}

// handleTestCase2 - Split [done] marker at the end
func (ms *MockServer) handleTestCase2(w http.ResponseWriter) {
	log.Println("Handling test case 2: Split [done] marker with thinking parts")

	// Thinking part
	thinkingData := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text":    "Analyzing the request and preparing response...",
							"thought": true,
						},
					},
				},
			},
		},
	}
	ms.writeSSEData(w, thinkingData)

	// Regular content chunks
	contentChunks := []string{
		"This is test case 2, which demonstrates splitting the [done] marker. ",
		"The response will be delivered normally, but the final [done] token ",
		"will be split across multiple chunks to test the proxy's ability ",
		"to handle partial markers. ",
		"Here comes the content ending with a split done marker: ",
	}

	for _, chunk := range contentChunks {
		contentData := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"text": chunk,
							},
						},
					},
				},
			},
		}
		ms.writeSSEData(w, contentData)
	}

	// Split the [done] marker across chunks
	doneChunks := []string{"[do", "ne]"}
	for i, chunk := range doneChunks {
		contentData := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"text": chunk,
							},
						},
					},
				},
			},
		}

		// Add finishReason to the last chunk
		if i == len(doneChunks)-1 {
			contentData["candidates"].([]map[string]interface{})[0]["finishReason"] = "STOP"
		}

		ms.writeSSEData(w, contentData)
	}

	log.Println("Test case 2 completed with split [done] marker")
}

// handleTestCase3 - Empty response
func (ms *MockServer) handleTestCase3(w http.ResponseWriter) {
	log.Println("Handling test case 3: Empty response")

	// Send a minimal response with just finishReason
	contentData := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text": "",
						},
					},
				},
				"finishReason": "STOP",
			},
		},
	}

	ms.writeSSEData(w, contentData)
	log.Println("Test case 3 completed with empty response")
}

// handleStreamingRequest handles streaming POST requests
func (ms *MockServer) handleStreamingRequest(w http.ResponseWriter, r *http.Request) {
	// Parse test case from URL path
	testCase := ms.extractTestCaseFromPath(r.URL.Path)

	log.Printf("Received streaming request for test case %d from path %s", testCase, r.URL.Path)

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	w.WriteHeader(http.StatusOK)

	// Handle different test cases
	switch TestCase(testCase) {
	case TestCaseNoEndMarker:
		ms.handleTestCase1(w)
	case TestCaseSplitEndMarker:
		ms.handleTestCase2(w)
	case TestCaseEmptyResponse:
		ms.handleTestCase3(w)
	}
}

// handleNonStreamingRequest handles regular POST requests
func (ms *MockServer) handleNonStreamingRequest(w http.ResponseWriter, r *http.Request) {
	// Parse test case from URL path
	testCase := ms.extractTestCaseFromPath(r.URL.Path)

	log.Printf("Received non-streaming request for test case %d from path %s", testCase, r.URL.Path)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	switch TestCase(testCase) {
	case TestCaseEmptyResponse:
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{}")
	default:
		response := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"text": fmt.Sprintf("This is a non-streaming response for test case %d. The content is delivered as a single JSON response instead of streaming chunks.", testCase),
							},
						},
					},
					"finishReason": "STOP",
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// handleCORS handles CORS preflight requests
func handleCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusOK)
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":  "healthy",
		"message": "Mock server is running",
		"testcases": map[string]string{
			"type-1": "No end marker with thinking parts",
			"type-2": "Split [done] marker with thinking parts",
			"type-3": "Empty response",
		},
		"usage": "Use path-based routing: /type-1, /type-2, or /type-3",
		"examples": []string{
			"/type-1/v1beta/models/gemini-pro:streamGenerateContent",
			"/type-2/v1beta/models/gemini-pro:streamGenerateContent",
			"/type-3/v1beta/models/gemini-pro:streamGenerateContent",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}

// ServeHTTP implements the main request handler
func (ms *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received %s request to %s", r.Method, r.URL.Path)

	if r.Method == "OPTIONS" {
		handleCORS(w, r)
		return
	}

	// Check if this is a streaming request based on URL path
	isStreaming := strings.Contains(r.URL.Path, "stream") ||
		r.URL.Query().Get("alt") == "sse" ||
		r.URL.Query().Get("stream") == "true"

	if r.Method == "POST" && isStreaming {
		ms.handleStreamingRequest(w, r)
	} else if r.Method == "POST" {
		ms.handleNonStreamingRequest(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Create mock server instance
	mockServer := NewMockServer()

	// Set up routes
	router := mux.NewRouter()

	// Health check endpoints
	router.HandleFunc("/health", healthHandler).Methods("GET")
	router.HandleFunc("/healthz", healthHandler).Methods("GET")

	// Handle all other requests with the mock server
	router.PathPrefix("/").Handler(mockServer)

	port := "8081"
	log.Printf("Starting mock server on port %s", port)
	log.Println("Available test cases:")
	log.Println("  type-1: No [done] marker with thinking parts")
	log.Println("  type-2: Split [done] marker with thinking parts")
	log.Println("  type-3: Empty response")
	log.Println("Usage: Use path-based routing with /type-1, /type-2, or /type-3")
	log.Printf("Examples:")
	log.Printf("  http://localhost:%s/type-1/v1beta/models/gemini-pro:streamGenerateContent", port)
	log.Printf("  http://localhost:%s/type-2/v1beta/models/gemini-pro:streamGenerateContent", port)
	log.Printf("  http://localhost:%s/type-3/v1beta/models/gemini-pro:streamGenerateContent", port)

	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
