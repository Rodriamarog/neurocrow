// sentiment.go
package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Analysis struct {
	Status     string  `json:"status"`      // "general", "need_human", or "frustrated"
	Confidence float64 `json:"confidence"`  // 0.0 to 1.0
	TokensUsed int     `json:"tokens_used"` // Total tokens used in request + response
}

type Config struct {
	FireworksKey string
	Timeout      time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		Timeout: 5 * time.Second,
	}
}

type Analyzer struct {
	config Config
	client *http.Client
}

func New(config Config) *Analyzer {
	return &Analyzer{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type FireworksRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	MaxTokens        int       `json:"max_tokens"`
	Temperature      float64   `json:"temperature"`
	TopP             float64   `json:"top_p"`
	TopK             int       `json:"top_k"`
	PresencePenalty  float64   `json:"presence_penalty"`
	FrequencyPenalty float64   `json:"frequency_penalty"`
}

type FireworksResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Analyze performs sentiment analysis on a message
func (a *Analyzer) Analyze(ctx context.Context, message string) (*Analysis, error) {
	systemPrompt := `You are a customer service message analyzer. Your task is to classify messages into exactly one of these three categories:
- "general" - for normal questions or comments
- "need_human" - when they explicitly ask for a human
- "frustrated" - when they show clear signs of frustration or the conversation is not being helpful

IMPORTANT:
1. Respond with ONLY ONE of these exact words: "general", "need_human", or "frustrated"
2. Do not add any other text, explanation, or punctuation
3. If ANY of these are true, respond with "frustrated":
   - The user shows clear frustration or anger
   - They use exclamation marks with complaints
   - They express that the conversation is not helpful
   - They make repeated requests in an agitated way
4. VERY IMPORTANT: If there's ANY sign of frustration, even if they ask for a human, respond with "frustrated"
5. Only respond with "need_human" if they politely ask for a human without showing frustration`

	// Prepare the request
	req := FireworksRequest{
		Model: "accounts/fireworks/models/llama-v3p1-8b-instruct",
		Messages: []Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: message,
			},
		},
		MaxTokens:        10,  // We only need a single word
		Temperature:      0.1, // Low temperature for consistency
		TopP:             1,
		TopK:             40,
		PresencePenalty:  0,
		FrequencyPenalty: 0,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.fireworks.ai/inference/v1/chat/completions",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.config.FireworksKey)

	// Send request
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read and log the response for debugging
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result FireworksResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %v, body: %s", err, string(respBody))
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response: %s", string(respBody))
	}

	// Clean and validate the response
	status := strings.TrimSpace(result.Choices[0].Message.Content)
	log.Printf("Raw LLM response: %q (Tokens used - Prompt: %d, Completion: %d, Total: %d)",
		status, result.Usage.PromptTokens, result.Usage.CompletionTokens, result.Usage.TotalTokens)

	switch status {
	case "general", "need_human", "frustrated":
		// Valid status
	default:
		return nil, fmt.Errorf("invalid status received: %q", status)
	}

	return &Analysis{
		Status:     status,
		Confidence: 0.95, // Fixed confidence since we use very low temperature
		TokensUsed: result.Usage.TotalTokens,
	}, nil
}
