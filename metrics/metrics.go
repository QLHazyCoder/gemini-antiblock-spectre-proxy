package metrics

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RequestEntry represents a single proxied request summary for UI display.
type RequestEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Upstream   string    `json:"upstreamUrl,omitempty"`
	Model      string    `json:"model"`
	Streaming  bool      `json:"streaming"`
	Antiblock  bool      `json:"antiblockEnabled"`
	Mode       string    `json:"handlingMode,omitempty"`
	DurationMs int64     `json:"durationMs"`
	Status     int       `json:"status"`
	Retries    int       `json:"retries"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	ClientIP   string    `json:"clientIp,omitempty"`
}

// Stats represents aggregated counters for display.
type Stats struct {
	TotalRequests int64     `json:"totalRequests"`
	RetryCount    int64     `json:"retryCount"`
	ErrorCount    int64     `json:"errorCount"`
	SuccessCount  int64     `json:"successCount"`
	LastActivity  time.Time `json:"lastActivity"`
}

// Snapshot is the top-level JSON returned to UI.
type Snapshot struct {
	Stats Stats          `json:"stats"`
	Logs  []RequestEntry `json:"logs"`
}

var (
	// counters
	totalRequests int64
	retryCount    int64
	errorCount    int64
	successCount  int64

	lastActivityMu sync.RWMutex
	lastActivity   time.Time

	// active sessions
	sessMu   sync.RWMutex
	sessions = map[string]*RequestEntry{}

	// ring buffer of completed requests for UI
	ringMu     sync.RWMutex
	ringBuf    = make([]RequestEntry, 0, 200)
	ringMaxLen = 200

	// SSE subscribers
	subMu       sync.RWMutex
	subscribers = map[chan []byte]struct{}{}
)

// StartRequest creates a session and bumps counters. Returns the request ID.
func StartRequest(r *http.Request, requestID string, isStreaming bool, model string, antiblockEnabled bool, handlingMode string) string {
	entry := &RequestEntry{
		ID:        requestID,
		Timestamp: time.Now().UTC(),
		Method:    r.Method,
		Path:      r.URL.Path,
		Model:     model,
		Streaming: isStreaming,
		Antiblock: antiblockEnabled,
		Mode:      handlingMode,
		ClientIP:  clientIPFromHeaders(r),
	}

	if entry.Model == "" {
		entry.Model = extractModelFromPath(r.URL.Path)
	}

	atomic.AddInt64(&totalRequests, 1)
	setLastActivity(time.Now().UTC())

	sessMu.Lock()
	sessions[requestID] = entry
	sessMu.Unlock()

	broadcastEvent(map[string]interface{}{
		"type":  "start",
		"entry": entry,
	})

	return requestID
}

func IncRetry(requestID string) {
	atomic.AddInt64(&retryCount, 1)
	sessMu.Lock()
	if s, ok := sessions[requestID]; ok {
		s.Retries++
	}
	sessMu.Unlock()

	broadcastEvent(map[string]interface{}{
		"type":      "retry",
		"requestId": requestID,
	})
}

// FinishRequest finalizes a session and appends it to the ring buffer.
func FinishRequest(requestID string, status int, success bool, errMsg string) {
	now := time.Now().UTC()
	setLastActivity(now)

	sessMu.Lock()
	s, ok := sessions[requestID]
	if ok {
		s.Status = status
		s.Success = success
		s.Error = errMsg
		s.DurationMs = now.Sub(s.Timestamp).Milliseconds()
		delete(sessions, requestID)
	}
	sessMu.Unlock()

	if success {
		atomic.AddInt64(&successCount, 1)
	} else {
		atomic.AddInt64(&errorCount, 1)
	}

	if ok {
		ringMu.Lock()
		// append and enforce max size
		ringBuf = append(ringBuf, *s)
		if len(ringBuf) > ringMaxLen {
			ringBuf = ringBuf[len(ringBuf)-ringMaxLen:]
		}
		ringMu.Unlock()

		broadcastEvent(map[string]interface{}{
			"type":  "finish",
			"entry": s,
		})
	}
}

// SetUpstream updates the recorded upstream URL for an active request.
func SetUpstream(requestID, upstream string) {
	normalized := normalizeUpstreamDisplay(upstream)

	sessMu.Lock()
	if s, ok := sessions[requestID]; ok {
		s.Upstream = normalized
	}
	sessMu.Unlock()
}

func normalizeUpstreamDisplay(raw string) string {
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		scheme := parsed.Scheme
		if scheme == "" {
			scheme = "https"
		}
		return scheme + "://" + parsed.Host
	}
	return raw
}

// Snapshot returns current stats and recent logs.
func GetSnapshot(limit int) Snapshot {
	stats := Stats{
		TotalRequests: atomic.LoadInt64(&totalRequests),
		RetryCount:    atomic.LoadInt64(&retryCount),
		ErrorCount:    atomic.LoadInt64(&errorCount),
		SuccessCount:  atomic.LoadInt64(&successCount),
		LastActivity:  getLastActivity(),
	}

	ringMu.RLock()
	logs := make([]RequestEntry, len(ringBuf))
	copy(logs, ringBuf)
	ringMu.RUnlock()

	if limit > 0 && len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}

	return Snapshot{Stats: stats, Logs: logs}
}

func extractModelFromPath(p string) string {
	// try to find after "/models/"
	idx := strings.Index(p, "/models/")
	if idx == -1 {
		return ""
	}
	s := p[idx+len("/models/"):]
	// model name may end at ':' or '/'
	for i, r := range s {
		if r == ':' || r == '/' {
			return s[:i]
		}
	}
	return s
}

func clientIPFromHeaders(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		// may contain multiple, take first
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func setLastActivity(t time.Time) {
	lastActivityMu.Lock()
	lastActivity = t
	lastActivityMu.Unlock()
}

func getLastActivity() time.Time {
	lastActivityMu.RLock()
	t := lastActivity
	lastActivityMu.RUnlock()
	return t
}

// SSE subscription management -------------------------------------------------

// Subscribe registers a new SSE subscriber channel.
func Subscribe() chan []byte {
	ch := make(chan []byte, 64)
	subMu.Lock()
	subscribers[ch] = struct{}{}
	subMu.Unlock()
	return ch
}

// Unsubscribe removes an SSE subscriber.
func Unsubscribe(ch chan []byte) {
	subMu.Lock()
	delete(subscribers, ch)
	close(ch)
	subMu.Unlock()
}

func broadcastEvent(v interface{}) {
	// best-effort, JSON encode once per broadcast
	data, _ := json.Marshal(v)
	line := append([]byte("data: "), data...)
	line = append(line, '\n', '\n')
	subMu.RLock()
	for ch := range subscribers {
		select {
		case ch <- line:
		default:
			// drop if subscriber is slow
		}
	}
	subMu.RUnlock()
}
