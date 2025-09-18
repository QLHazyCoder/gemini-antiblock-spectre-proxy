package logger

import (
	"fmt"
	"log"
	"time"
)

var debugMode bool

// SetDebugMode sets whether debug logging is enabled
func SetDebugMode(enabled bool) {
	debugMode = enabled
}

// LogDebug logs debug messages (only if debug mode is enabled)
func LogDebug(args ...interface{}) {
	if debugMode {
		log.Printf("[DEBUG %s] %s", time.Now().Format(time.RFC3339), fmt.Sprint(args...))
	}
}

// LogInfo logs info messages
func LogInfo(args ...interface{}) {
	log.Printf("[INFO %s] %s", time.Now().Format(time.RFC3339), fmt.Sprint(args...))
}

// LogError logs error messages
func LogError(args ...interface{}) {
	log.Printf("[ERROR %s] %s", time.Now().Format(time.RFC3339), fmt.Sprint(args...))
}
