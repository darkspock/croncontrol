// Package orchestra implements the AI Director for CronControl Orchestras.
//
// The AI Director is a built-in Go component that uses LLM APIs to make
// autonomous decisions about what to do next in an orchestra workflow.
// Supports multiple providers: Anthropic (Claude), OpenAI (GPT), Google (Gemini).
package orchestra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Tool represents a function the AI can call.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

// ToolCall represents the AI requesting to call a tool.
type ToolCall struct {
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// ProviderResponse is the result of calling an LLM.
type ProviderResponse struct {
	Text      string     `json:"text"`
	ToolCalls []ToolCall `json:"tool_calls"`
	StopReason string   `json:"stop_reason"`
}

// Provider is the interface for LLM API providers.
type Provider interface {
	Call(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ProviderResponse, error)
	Name() string
}

// Message is a chat message for the LLM.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ============================================================================
// Anthropic Provider (Claude)
// ============================================================================

type AnthropicProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &AnthropicProvider{apiKey: apiKey, model: model, client: &http.Client{Timeout: 60 * time.Second}}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) Call(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ProviderResponse, error) {
	// Convert tools to Anthropic format
	var anthropicTools []map[string]any
	for _, t := range tools {
		anthropicTools = append(anthropicTools, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.InputSchema,
		})
	}

	// Convert messages
	var anthropicMsgs []map[string]any
	for _, m := range messages {
		anthropicMsgs = append(anthropicMsgs, map[string]any{"role": m.Role, "content": m.Content})
	}

	body, _ := json.Marshal(map[string]any{
		"model":      p.model,
		"max_tokens": 4096,
		"system":     systemPrompt,
		"messages":   anthropicMsgs,
		"tools":      anthropicTools,
	})

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content    []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	json.Unmarshal(respBody, &result)

	response := &ProviderResponse{StopReason: result.StopReason}
	for _, c := range result.Content {
		if c.Type == "text" {
			response.Text += c.Text
		} else if c.Type == "tool_use" {
			response.ToolCalls = append(response.ToolCalls, ToolCall{Name: c.Name, Input: c.Input})
		}
	}
	return response, nil
}

// ============================================================================
// OpenAI Provider (GPT)
// ============================================================================

type OpenAIProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAIProvider{apiKey: apiKey, model: model, client: &http.Client{Timeout: 60 * time.Second}}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Call(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ProviderResponse, error) {
	var openaiTools []map[string]any
	for _, t := range tools {
		openaiTools = append(openaiTools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.InputSchema,
			},
		})
	}

	var openaiMsgs []map[string]any
	openaiMsgs = append(openaiMsgs, map[string]any{"role": "system", "content": systemPrompt})
	for _, m := range messages {
		openaiMsgs = append(openaiMsgs, map[string]any{"role": m.Role, "content": m.Content})
	}

	body, _ := json.Marshal(map[string]any{
		"model":    p.model,
		"messages": openaiMsgs,
		"tools":    openaiTools,
	})

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	json.Unmarshal(respBody, &result)

	response := &ProviderResponse{}
	if len(result.Choices) > 0 {
		choice := result.Choices[0]
		response.Text = choice.Message.Content
		response.StopReason = choice.FinishReason
		for _, tc := range choice.Message.ToolCalls {
			var input map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &input)
			response.ToolCalls = append(response.ToolCalls, ToolCall{Name: tc.Function.Name, Input: input})
		}
	}
	return response, nil
}

// ============================================================================
// Google Provider (Gemini)
// ============================================================================

type GoogleProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewGoogleProvider(apiKey, model string) *GoogleProvider {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GoogleProvider{apiKey: apiKey, model: model, client: &http.Client{Timeout: 60 * time.Second}}
}

func (p *GoogleProvider) Name() string { return "google" }

func (p *GoogleProvider) Call(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ProviderResponse, error) {
	var geminiTools []map[string]any
	for _, t := range tools {
		geminiTools = append(geminiTools, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.InputSchema,
		})
	}

	var contents []map[string]any
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{
			"role":  role,
			"parts": []map[string]any{{"text": m.Content}},
		})
	}

	body, _ := json.Marshal(map[string]any{
		"contents":         contents,
		"systemInstruction": map[string]any{"parts": []map[string]any{{"text": systemPrompt}}},
		"tools":            []map[string]any{{"functionDeclarations": geminiTools}},
	})

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string         `json:"name"`
						Args map[string]any `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	json.Unmarshal(respBody, &result)

	response := &ProviderResponse{}
	if len(result.Candidates) > 0 {
		for _, part := range result.Candidates[0].Content.Parts {
			if part.Text != "" {
				response.Text += part.Text
			}
			if part.FunctionCall != nil {
				response.ToolCalls = append(response.ToolCalls, ToolCall{Name: part.FunctionCall.Name, Input: part.FunctionCall.Args})
			}
		}
	}
	return response, nil
}

// NewProvider creates the appropriate provider based on name.
func NewProvider(providerName, apiKey, model string) (Provider, error) {
	switch providerName {
	case "anthropic":
		return NewAnthropicProvider(apiKey, model), nil
	case "openai":
		return NewOpenAIProvider(apiKey, model), nil
	case "google":
		return NewGoogleProvider(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unknown AI provider: %s", providerName)
	}
}
