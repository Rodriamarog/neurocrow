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
		Timeout: 15 * time.Second,
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
	systemPrompt := `You're a sentiment analysis agent. Respond with exactly one word:

"general" - for normal questions, requests, complaints, or any messages that don't EXPLICITLY ask for human help

"need_human" - ONLY if they EXPLICITLY and DIRECTLY ask to speak to a human, agent, representative, or person. Examples: "I want to talk to a human", "Can I speak to a person?", "Transfer me to an agent", "I need human help"

"frustrated" - ONLY if they explicitly express anger, frustration, or complaints

IMPORTANT: Be very conservative with "need_human". Most complaints, problems, or even expressions of frustration should be "general" or "frustrated", NOT "need_human" unless they specifically ask to talk to a person.

Most of the time, the message will be "general". If you are not sure, respond with "general".

Remember, only respond with one of these three words: general, need_human, frustrated

Message to analyse:`

	// Prepare the request
	req := FireworksRequest{
		Model: "accounts/fireworks/models/llama4-maverick-instruct-basic",
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
	rawStatus := strings.TrimSpace(result.Choices[0].Message.Content)
	log.Printf("Raw LLM response: %q (Tokens used - Prompt: %d, Completion: %d, Total: %d)",
		rawStatus, result.Usage.PromptTokens, result.Usage.CompletionTokens, result.Usage.TotalTokens)

	// Normalize the status - convert to lowercase and clean up
	status := strings.ToLower(rawStatus)
	status = strings.ReplaceAll(status, "\"", "") // Remove quotes
	status = strings.ReplaceAll(status, "'", "")  // Remove single quotes
	status = strings.ReplaceAll(status, ".", "")  // Remove periods
	status = strings.Split(status, " ")[0]        // Take only the first word
	status = strings.Split(status, "\n")[0]       // Take only the first line

	log.Printf("Normalized status: %q", status)

	// Map variations to standard values (all lowercase now)
	switch status {
	case "general", "normal", "neutral", "regular":
		status = "general"
	case "need_human", "needhuman", "human", "agent", "need-human", "need_human_help":
		status = "need_human"
	case "frustrated", "angry", "upset", "mad", "annoyed", "irritated":
		status = "frustrated"
	default:
		log.Printf("⚠️ Unexpected status received: %q (raw: %q), defaulting to 'general'", status, rawStatus)
		status = "general" // Default to general instead of erroring out
	}

	return &Analysis{
		Status:     status,
		Confidence: 0.95, // Fixed confidence since we use very low temperature
		TokensUsed: result.Usage.TotalTokens,
	}, nil
}
