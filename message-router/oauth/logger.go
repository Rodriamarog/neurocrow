// logger.go
package oauth

import (
	"log"
)

// OAuth-specific logging functions with clear prefixes
// This keeps OAuth logs separate from message routing logs

func LogError(format string, args ...interface{}) {
	log.Printf("🔐❌ OAUTH: "+format, args...)
}

func LogInfo(format string, args ...interface{}) {
	log.Printf("🔐ℹ️ OAUTH: "+format, args...)
}

func LogDebug(format string, args ...interface{}) {
	log.Printf("🔐🔍 OAUTH: "+format, args...)
}