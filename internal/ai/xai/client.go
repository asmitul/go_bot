package xai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go_bot/internal/config"
	"go_bot/internal/logger"
)

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type Option func(*Client)

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

func NewClient(cfg config.XAIConfig, opts ...Option) (*Client, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("xai api key is empty")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.x.ai/v1"
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "grok-beta"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	client := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

type chatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []chatCompletionMessage `json:"messages"`
	Temperature float64                 `json:"temperature"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
	Stream      bool                    `json:"stream"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type orderExtractionResult struct {
	OrderNumbers []string `json:"order_numbers"`
}

func (c *Client) ExtractOrderNumbers(ctx context.Context, text string) ([]string, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}

	systemPrompt := "You are an assistant that extracts potential merchant order numbers from arbitrary text, captions, filenames, or logs. Respond ONLY with compact JSON like {\"order_numbers\":[\"...\"]}. Include numbers even if you are not fully sure but they resemble an order identifier. Do not add explanations."

	userPrompt := fmt.Sprintf("消息内容:\n%s", text)

	payload := chatCompletionRequest{
		Model: c.model,
		Messages: []chatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal xai request failed: %w", err)
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create xai request failed: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request xai api failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read xai response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.L().Warnf("xAI response: status=%d body=%s", resp.StatusCode, truncate(string(data), 512))
		return nil, fmt.Errorf("xai http error: status=%d", resp.StatusCode)
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("decode xai response failed: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("xai response has no choices")
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	result, err := parseOrderNumbersFromContent(content)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func parseOrderNumbersFromContent(content string) ([]string, error) {
	if content == "" {
		return nil, nil
	}

	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
	}

	var result orderExtractionResult
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return nil, fmt.Errorf("decode xai order payload failed: %w", err)
	}

	orders := make([]string, 0, len(result.OrderNumbers))
	for _, item := range result.OrderNumbers {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		orders = append(orders, item)
	}
	return orders, nil
}

func truncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit]
}
