package handlers

import (
	"bytes"
	gz "compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"gemini-antiblock/config"
	"gemini-antiblock/logger"
	"gemini-antiblock/metrics"
	"gemini-antiblock/streaming"
)

// ProxyHandler handles proxy requests to Gemini API
type ProxyHandler struct {
	Config      *config.Config
	RateLimiter *RateLimiter
	rrCounter   uint64
}

const (
	handlingModeAntiblockStream   = "antiblock-stream"
	handlingModePassthroughStream = "passthrough-stream"
	handlingModeStreamOther       = "stream"
	handlingModeNonStream         = "non-stream"
)

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(cfg *config.Config, rateLimiter *RateLimiter) *ProxyHandler {
	return &ProxyHandler{
		Config:      cfg,
		RateLimiter: rateLimiter,
	}
}

// BuildUpstreamHeaders builds headers for upstream requests
func (h *ProxyHandler) BuildUpstreamHeaders(reqHeaders http.Header) http.Header {
	headers := make(http.Header)

	// Copy specific headers
	if auth := reqHeaders.Get("Authorization"); auth != "" {
		headers.Set("Authorization", auth)
	}
	if apiKey := reqHeaders.Get("X-Goog-Api-Key"); apiKey != "" {
		headers.Set("X-Goog-Api-Key", apiKey)
	}
	if contentType := reqHeaders.Get("Content-Type"); contentType != "" {
		headers.Set("Content-Type", contentType)
	}
	if accept := reqHeaders.Get("Accept"); accept != "" {
		headers.Set("Accept", accept)
	}

	return headers
}

func extractModelIdentifier(path string) string {
	trimmed := strings.TrimPrefix(path, "/")
	segments := strings.Split(trimmed, "/")
	for i, seg := range segments {
		switch strings.ToLower(seg) {
		case "models", "tunedmodels":
			if i+1 < len(segments) {
				candidate := segments[i+1]
				for j, r := range candidate {
					if r == ':' {
						return candidate[:j]
					}
				}
				return candidate
			}
		}
	}
	return ""
}

func (h *ProxyHandler) isAntiblockTarget(model string) bool {
	if model == "" {
		return false
	}
	for _, prefix := range h.Config.AntiblockModelPrefixes {
		if prefix != "" && strings.HasPrefix(model, prefix) {
			return true
		}
	}
	return false
}

// InjectSystemPrompt injects a system prompt to ensure the [done] token is present.
// It intelligently handles both system_instruction (snake_case) and systemInstruction (camelCase)
// by merging the content of system_instruction into systemInstruction before processing.
// systemInstruction is the officially recommended format.
func (h *ProxyHandler) InjectSystemPrompt(body map[string]interface{}) {
	newSystemPromptPart := map[string]interface{}{
		"text": "IMPORTANT: At the very end of your entire response, you must write the token [done] to signal completion. This is a mandatory technical requirement.",
	}

	// Standardize: If system_instruction exists, merge its content into systemInstruction.
	if snakeVal, snakeExists := body["system_instruction"]; snakeExists {
		// Ensure camelCase map exists
		camelMap, _ := body["systemInstruction"].(map[string]interface{})
		if camelMap == nil {
			camelMap = make(map[string]interface{})
		}

		// Ensure camelCase parts array exists
		camelParts, _ := camelMap["parts"].([]interface{})
		if camelParts == nil {
			camelParts = make([]interface{}, 0)
		}

		// If snake_case is a valid map with its own parts, prepend them to camelCase parts
		if snakeMap, snakeOk := snakeVal.(map[string]interface{}); snakeOk {
			if snakeParts, snakePartsOk := snakeMap["parts"].([]interface{}); snakePartsOk {
				camelParts = append(snakeParts, camelParts...)
			}
		}

		// Update the camelCase field with the merged parts and delete the snake_case one
		camelMap["parts"] = camelParts
		body["systemInstruction"] = camelMap
		delete(body, "system_instruction")
	}

	// --- From this point on, we only need to deal with systemInstruction ---

	// Case 1: systemInstruction field is missing or null. Create it.
	if val, exists := body["systemInstruction"]; !exists || val == nil {
		body["systemInstruction"] = map[string]interface{}{
			"parts": []interface{}{newSystemPromptPart},
		}
		return
	}

	instruction, ok := body["systemInstruction"].(map[string]interface{})
	if !ok {
		// The field exists but is of the wrong type. Overwrite it.
		body["systemInstruction"] = map[string]interface{}{
			"parts": []interface{}{newSystemPromptPart},
		}
		return
	}

	// Case 2: The instruction field exists, but its 'parts' array is missing, null, or not an array.
	parts, ok := instruction["parts"].([]interface{})
	if !ok {
		instruction["parts"] = []interface{}{newSystemPromptPart}
		return
	}

	// Case 3: The instruction field and its 'parts' array both exist. Append to the existing array.
	instruction["parts"] = append(parts, newSystemPromptPart)
}

// HandleStreamingPost handles streaming POST requests
func (h *ProxyHandler) HandleStreamingPost(w http.ResponseWriter, r *http.Request) {
	urlObj, _ := url.Parse(r.URL.String())
	upstreamBase := h.selectUpstreamBase()
	upstreamURL := upstreamBase + urlObj.Path
	if urlObj.RawQuery != "" {
		upstreamURL += "?" + urlObj.RawQuery
	}

	if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok && rid != "" {
		metrics.SetUpstream(rid, upstreamURL)
	}

	logger.LogInfo("=== NEW STREAMING REQUEST ===")
	logger.LogInfo("Upstream URL:", upstreamURL)
	logger.LogInfo("Request method:", r.Method)
	logger.LogInfo("Content-Type:", r.Header.Get("Content-Type"))

	// Read and parse request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.LogError("Failed to read request body:", err)
		JSONError(w, 400, "Failed to read request body", err.Error())
		return
	}

	var requestBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
		logger.LogError("Failed to parse request body:", err)
		JSONError(w, 400, "Invalid JSON in request body", err.Error())
		return
	}

	logger.LogDebug(fmt.Sprintf("Request body size: %d bytes", len(bodyBytes)))

	if contents, ok := requestBody["contents"].([]interface{}); ok {
		logger.LogDebug(fmt.Sprintf("Parsed request body with %d messages", len(contents)))
	}

	// Inject system prompt
	h.InjectSystemPrompt(requestBody)

	// Create upstream request
	modifiedBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		logger.LogError("Failed to marshal modified request body:", err)
		JSONError(w, 500, "Internal server error", "Failed to process request body")
		return
	}

	logger.LogInfo("=== MAKING INITIAL REQUEST ===")
	upstreamHeaders := h.BuildUpstreamHeaders(r.Header)

	upstreamReq, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(modifiedBodyBytes))
	if err != nil {
		logger.LogError("Failed to create upstream request:", err)
		JSONError(w, 500, "Internal server error", "Failed to create upstream request")
		return
	}

	upstreamReq.Header = upstreamHeaders

	client := &http.Client{}
	initialResponse, err := client.Do(upstreamReq)
	if err != nil {
		logger.LogError("Failed to make initial request:", err)
		JSONError(w, 502, "Bad Gateway", "Failed to connect to upstream server")
		if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
			metrics.FinishRequest(rid, 502, false, "connect upstream failed")
		}
		return
	}

	logger.LogInfo(fmt.Sprintf("Initial response status: %d %s", initialResponse.StatusCode, initialResponse.Status))

	// Initial failure: return standardized error
	if initialResponse.StatusCode != http.StatusOK {
		logger.LogError("=== INITIAL REQUEST FAILED ===")
		logger.LogError("Status:", initialResponse.StatusCode)
		logger.LogError("Status Text:", initialResponse.Status)

		// Read error response
		errorBody, _ := io.ReadAll(initialResponse.Body)
		initialResponse.Body.Close()

		// Mark metrics as failure for this request
		if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
			metrics.FinishRequest(rid, initialResponse.StatusCode, false, string(errorBody))
		}

		// Try to parse as JSON error
		var errorResp map[string]interface{}
		if json.Unmarshal(errorBody, &errorResp) == nil {
			if errorObj, ok := errorResp["error"].(map[string]interface{}); ok {
				if _, hasStatus := errorObj["status"]; !hasStatus {
					if code, ok := errorObj["code"].(float64); ok {
						errorObj["status"] = StatusToGoogleStatus(int(code))
					}
				}
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(initialResponse.StatusCode)
			json.NewEncoder(w).Encode(errorResp)
			return
		}

		// Fallback to standard error
		message := "Request failed"
		if initialResponse.StatusCode == 429 {
			message = "Resource has been exhausted (e.g. check quota)."
		}
		JSONError(w, initialResponse.StatusCode, message, string(errorBody))
		return
	}

	logger.LogInfo("=== INITIAL REQUEST SUCCESSFUL - STARTING STREAM PROCESSING ===")

	// Set up streaming response
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Additional headers to prevent buffering by proxies
	w.Header().Set("X-Accel-Buffering", "no") // Nginx
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	w.WriteHeader(http.StatusOK)

	// Process stream with retry logic
	requestID, _ := r.Context().Value(ctxKeyRequestID).(string)
	err = streaming.ProcessStreamAndRetryInternally(
		h.Config,
		initialResponse.Body,
		w,
		requestBody,
		upstreamURL,
		r.Header,
		requestID,
	)

	if err != nil {
		logger.LogError("=== UNHANDLED EXCEPTION IN STREAM PROCESSOR ===")
		logger.LogError("Exception:", err)
		// when stream processor returns error, mark failure if not already
		if requestID != "" {
			// classify status best-effort
			status := 500
			if err == streaming.ErrRetryLimitExceeded {
				status = 504
			}
			metrics.FinishRequest(requestID, status, false, err.Error())
		}
	} else {
		if requestID != "" {
			metrics.FinishRequest(requestID, http.StatusOK, true, "")
		}
	}

	initialResponse.Body.Close()
	logger.LogInfo("Streaming response completed")
}

// HandleStreamingPassthrough forwards streaming requests without antiblock processing
func (h *ProxyHandler) HandleStreamingPassthrough(w http.ResponseWriter, r *http.Request) {
	urlObj, _ := url.Parse(r.URL.String())
	upstreamBase := h.selectUpstreamBase()
	upstreamURL := upstreamBase + urlObj.Path
	if urlObj.RawQuery != "" {
		upstreamURL += "?" + urlObj.RawQuery
	}

	if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok && rid != "" {
		metrics.SetUpstream(rid, upstreamURL)
	}

	logger.LogInfo("=== STREAMING PASSTHROUGH REQUEST ===")
	logger.LogInfo("[PASSTHROUGH] Upstream URL:", upstreamURL)

	upstreamHeaders := h.BuildUpstreamHeaders(r.Header)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		logger.LogError("[PASSTHROUGH] Failed to create upstream request:", err)
		JSONError(w, 500, "Internal server error", "Failed to create upstream request")
		if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
			metrics.FinishRequest(rid, 500, false, err.Error())
		}
		return
	}
	upstreamReq.Header = upstreamHeaders

	client := &http.Client{}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		logger.LogError("[PASSTHROUGH] Failed to connect to upstream server:", err)
		JSONError(w, 502, "Bad Gateway", "Failed to connect to upstream server")
		if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
			metrics.FinishRequest(rid, 502, false, err.Error())
		}
		return
	}
	defer resp.Body.Close()

	logger.LogInfo("[PASSTHROUGH] Initial response status:", resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		preview := errorBody
		if len(preview) > 800 {
			preview = preview[:800]
		}
		logger.LogDebug("[PASSTHROUGH] Upstream error body prefix:", string(preview))
		JSONError(w, resp.StatusCode, resp.Status, string(errorBody))
		if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
			metrics.FinishRequest(rid, resp.StatusCode, false, string(errorBody))
		}
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.LogError("[PASSTHROUGH] Response writer does not support streaming flush")
		JSONError(w, 500, "Internal server error", "Streaming not supported by response writer")
		if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
			metrics.FinishRequest(rid, 500, false, "response writer cannot flush")
		}
		return
	}

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(resp.StatusCode)

	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				logger.LogError("[PASSTHROUGH] Failed to write downstream chunk:", writeErr)
				if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
					metrics.FinishRequest(rid, http.StatusBadGateway, false, writeErr.Error())
				}
				return
			}
			flusher.Flush()
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			logger.LogError("[PASSTHROUGH] Upstream read error:", readErr)
			if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
				metrics.FinishRequest(rid, http.StatusBadGateway, false, readErr.Error())
			}
			return
		}
	}

	logger.LogInfo("[PASSTHROUGH] Stream completed successfully")
	if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
		metrics.FinishRequest(rid, resp.StatusCode, true, "")
	}
}

// HandleNonStreaming handles non-streaming requests
func (h *ProxyHandler) HandleNonStreaming(w http.ResponseWriter, r *http.Request) {
	urlObj, _ := url.Parse(r.URL.String())
	upstreamBase := h.selectUpstreamBase()
	upstreamURL := upstreamBase + urlObj.Path
	if urlObj.RawQuery != "" {
		upstreamURL += "?" + urlObj.RawQuery
	}

	if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok && rid != "" {
		metrics.SetUpstream(rid, upstreamURL)
	}

	upstreamHeaders := h.BuildUpstreamHeaders(r.Header)

	// 可观测：打印上游 URL
	logger.LogInfo("[NON-STREAM] Upstream URL:", upstreamURL)

	var body io.Reader
	if r.Method != "GET" && r.Method != "HEAD" {
		body = r.Body
	}

	upstreamReq, err := http.NewRequest(r.Method, upstreamURL, body)
	if err != nil {
		JSONError(w, 500, "Internal server error", "Failed to create upstream request")
		return
	}
	upstreamReq.Header = upstreamHeaders

	client := &http.Client{}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		JSONError(w, 502, "Bad Gateway", "Failed to connect to upstream server")
		if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
			metrics.FinishRequest(rid, 502, false, "connect upstream failed")
		}
		return
	}
	defer resp.Body.Close()

	// 可观测：记录初始返回码
	logger.LogInfo("[NON-STREAM] Initial response status:", resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK {
		// 非 200：读取错误体
		errorBody, _ := io.ReadAll(resp.Body)

		// 可观测：错误体前缀（最多 800 字节）
		preview := errorBody
		if len(preview) > 800 {
			preview = preview[:800]
		}
		logger.LogDebug("[NON-STREAM] Upstream error body prefix:", string(preview))

		// 尝试标准化为 Google 风格 error JSON
		var errorResp map[string]interface{}
		if json.Unmarshal(errorBody, &errorResp) == nil {
			if errorObj, ok := errorResp["error"].(map[string]interface{}); ok {
				if _, hasStatus := errorObj["status"]; !hasStatus {
					if code, ok := errorObj["code"].(float64); ok {
						errorObj["status"] = StatusToGoogleStatus(int(code))
					}
				}
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(resp.StatusCode)
			json.NewEncoder(w).Encode(errorResp)
			return
		}

		// Fallback：非 JSON 错误
		JSONError(w, resp.StatusCode, resp.Status, string(errorBody))
		if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
			metrics.FinishRequest(rid, resp.StatusCode, false, string(errorBody))
		}
		return
	}

	// 200 成功：读全体 → sniff gzip → 解压 → 过滤头 → 回写明文 JSON
	raw, _ := io.ReadAll(resp.Body)
	if (len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b) || strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		if gr, err := gz.NewReader(bytes.NewReader(raw)); err == nil {
			if dec, err2 := io.ReadAll(gr); err2 == nil {
				raw = dec
			}
			gr.Close()
		}
	}

	// 过滤 Content-Encoding/Content-Length，避免下游遇到 gzip 或长度不匹配
	for name, values := range resp.Header {
		ln := strings.ToLower(name)
		if ln == "content-encoding" || ln == "content-length" {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(resp.StatusCode)
	w.Write(raw)
	if rid, ok := r.Context().Value(ctxKeyRequestID).(string); ok {
		metrics.FinishRequest(rid, resp.StatusCode, true, "")
	}
}

// ServeHTTP implements the http.Handler interface
var reqSeq int64

type contextKey string

const ctxKeyRequestID contextKey = "gemini-request-id"

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// First, enforce rate limiting if enabled and a key is present.
	if h.Config.EnableRateLimit {
		apiKey := r.Header.Get("X-Goog-Api-Key")
		if apiKey == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey != "" {
			logger.LogDebug("Enforcing rate limit for key ending with: ...", apiKey[len(apiKey)-4:])
			h.RateLimiter.Wait(apiKey)
			logger.LogDebug("Rate limit check passed for key.")
		}
	}

	logger.LogInfo("=== WORKER REQUEST ===")
	logger.LogInfo("Method:", r.Method)
	logger.LogInfo("URL:", r.URL.String())
	logger.LogInfo("User-Agent:", r.Header.Get("User-Agent"))
	logger.LogInfo("X-Forwarded-For:", r.Header.Get("X-Forwarded-For"))

	if r.Method == "OPTIONS" {
		logger.LogDebug("Handling CORS preflight request")
		HandleCORS(w, r)
		return
	}

	// Determine if this is a streaming request
	isStream := strings.Contains(strings.ToLower(r.URL.Path), "stream") ||
		strings.Contains(strings.ToLower(r.URL.Path), "sse") ||
		r.URL.Query().Get("alt") == "sse"

	model := extractModelIdentifier(r.URL.Path)
	antiblockEnabled := false
	handlingMode := handlingModeNonStream

	if isStream {
		if strings.EqualFold(r.Method, "POST") {
			if h.isAntiblockTarget(model) {
				antiblockEnabled = true
				handlingMode = handlingModeAntiblockStream
			} else {
				handlingMode = handlingModePassthroughStream
			}
		} else {
			handlingMode = handlingModeStreamOther
		}
	}

	logger.LogInfo("Detected streaming request:", isStream)
	logger.LogInfo("Resolved model identifier:", model)
	logger.LogInfo("Antiblock enabled:", antiblockEnabled)
	logger.LogInfo("Handling mode:", handlingMode)

	// start metrics session for this request
	rid := fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddInt64(&reqSeq, 1))
	metrics.StartRequest(r, rid, isStream, model, antiblockEnabled, handlingMode)
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, rid))

	if isStream {
		if strings.EqualFold(r.Method, "POST") {
			if antiblockEnabled {
				h.HandleStreamingPost(w, r)
				return
			}
			logger.LogInfo("Routing streaming request through passthrough handler (no antiblock)")
			h.HandleStreamingPassthrough(w, r)
			return
		}

		logger.LogInfo("Routing non-POST streaming request through passthrough handler")
		h.HandleStreamingPassthrough(w, r)
		return
	}

	h.HandleNonStreaming(w, r)
}

func (h *ProxyHandler) selectUpstreamBase() string {
	if bases := h.Config.UpstreamURLBases; len(bases) > 0 {
		idx := atomic.AddUint64(&h.rrCounter, 1) - 1
		selected := bases[int(idx%uint64(len(bases)))]
		logger.LogDebug(fmt.Sprintf("Selected Spectre upstream[%d]: %s", int(idx%uint64(len(bases))), selected))
		return selected
	}
	return h.Config.UpstreamURLBase
}
