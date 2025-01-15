// message-router/sentiment/sentiment_test.go
package sentiment

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func init() {
	// Try to load .env from the message-router directory
	currentDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Look for .env in current directory and parent directory
	envPaths := []string{
		filepath.Join(currentDir, ".env"),
		filepath.Join(filepath.Dir(currentDir), ".env"),
	}

	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}
}

func TestSentimentAnalyzer(t *testing.T) {
	// Get API key from environment
	apiKey := os.Getenv("FIREWORKS_API_KEY")
	if apiKey == "" {
		t.Fatal("FIREWORKS_API_KEY not set in .env file")
	}

	// Create config
	config := DefaultConfig()
	config.FireworksKey = apiKey

	// Create analyzer
	analyzer := New(config)

	// Test cases
	tests := []struct {
		name    string
		message string
		want    string // Expected status
		wantErr bool
	}{
		{
			name:    "General inquiry",
			message: "¿Cuál es el horario de atención?",
			want:    "general",
		},
		{
			name:    "Human request",
			message: "Necesito hablar con una persona por favor",
			want:    "need_human",
		},
		{
			name:    "Frustrated user",
			message: "Ya te dije tres veces que ese no es mi problema! No me estás entendiendo!",
			want:    "frustrated",
		},
		{
			name:    "Multiple human requests",
			message: "Por favor conectame con un agente. NECESITO HABLAR CON ALGUIEN YA!",
			want:    "frustrated", // Prioritizes frustration over human request
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add delay between tests to respect rate limits
			time.Sleep(500 * time.Millisecond)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			t.Logf("Testing message: %q", tt.message)
			analysis, err := analyzer.Analyze(ctx, tt.message)

			if (err != nil) != tt.wantErr {
				t.Errorf("Analyze() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && analysis.Status != tt.want {
				t.Errorf("Analyze() got status = %v, want %v", analysis.Status, tt.want)
			}

			if analysis != nil {
				t.Logf("Result: status=%s confidence=%.2f tokens=%d (≈$%.5f)",
					analysis.Status,
					analysis.Confidence,
					analysis.TokensUsed,
					float64(analysis.TokensUsed)*0.20/1_000_000) // $0.20 per 1M tokens
			}
		})
	}
}

// TestCustomMessages allows manual testing of specific messages
func TestCustomMessages(t *testing.T) {
	apiKey := os.Getenv("FIREWORKS_API_KEY")
	if apiKey == "" {
		t.Fatal("FIREWORKS_API_KEY not set in .env file")
	}

	config := DefaultConfig()
	config.FireworksKey = apiKey
	analyzer := New(config)

	messages := []string{
		"No me estás entendiendo, esto es inútil",
		"Quiero cambiar mi plan",
		"CONECTAME CON UN HUMANO YA!!!!",
		"La verdad que este bot no sirve para nada",
		"¿Pueden decirme cuánto cuesta la suscripción?",
	}

	for _, msg := range messages {
		// Add delay between tests to respect rate limits
		time.Sleep(500 * time.Millisecond)

		t.Logf("\nTesting message: %q", msg)

		analysis, err := analyzer.Analyze(context.Background(), msg)
		if err != nil {
			t.Errorf("Error analyzing message: %v", err)
			continue
		}

		t.Logf("Result: status=%s confidence=%.2f", analysis.Status, analysis.Confidence)
	}
}
