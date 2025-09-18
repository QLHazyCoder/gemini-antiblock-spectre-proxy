package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration values
type Config struct {
	UpstreamURLBase            string
	AntiblockModelPrefixes     []string
	SpectreProxyWorkerURL      string
	SpectreProxyAuthToken      string
	MaxConsecutiveRetries      int
	DebugMode                  bool
	RetryDelayMs               time.Duration
	SwallowThoughtsAfterRetry  bool
	Port                       string
	EnableRateLimit            bool
	RateLimitCount             int
	RateLimitWindowSeconds     int
	EnablePunctuationHeuristic bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	workerURL := getEnvString("SPECTRE_PROXY_WORKER_URL", "")
	authToken := getEnvString("SPECTRE_PROXY_AUTH_TOKEN", "")

	upstreamBase := getEnvString("UPSTREAM_URL_BASE", "")
	if upstreamBase == "" && workerURL != "" && authToken != "" {
		upstreamBase = buildSpectreUpstream(workerURL, authToken)
	}
	if upstreamBase == "" {
		upstreamBase = "https://generativelanguage.googleapis.com"
	}

	return &Config{
		UpstreamURLBase:            upstreamBase,
		AntiblockModelPrefixes:     getEnvStringSlice("ANTIBLOCK_MODEL_PREFIXES", []string{"gemini-2.5-pro"}),
		SpectreProxyWorkerURL:      workerURL,
		SpectreProxyAuthToken:      authToken,
		Port:                       getEnvString("PORT", "8080"),
		DebugMode:                  getEnvBool("DEBUG_MODE", true),
		MaxConsecutiveRetries:      getEnvInt("MAX_CONSECUTIVE_RETRIES", 100),
		RetryDelayMs:               time.Duration(getEnvInt("RETRY_DELAY_MS", 750)) * time.Millisecond,
		SwallowThoughtsAfterRetry:  getEnvBool("SWALLOW_THOUGHTS_AFTER_RETRY", true),
		EnableRateLimit:            getEnvBool("ENABLE_RATE_LIMIT", false),
		RateLimitCount:             getEnvInt("RATE_LIMIT_COUNT", 10),
		RateLimitWindowSeconds:     getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60),
		EnablePunctuationHeuristic: getEnvBool("ENABLE_PUNCTUATION_HEURISTIC", true),
	}
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

func buildSpectreUpstream(worker, token string) string {
	worker = strings.TrimSuffix(worker, "/")
	token = strings.Trim(token, "/")
	if worker == "" || token == "" {
		return ""
	}
	return worker + "/" + token + "/gemini"
}
