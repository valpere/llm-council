package openrouter

import (
	"context"

	"github.com/valpere/llm-council/internal/council"
)

// Client sends completion requests to the OpenRouter API.
// Full HTTP implementation is provided in a later milestone.
type Client struct {
	apiKey string
}

// NewClient creates a Client for the given OpenRouter API key.
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey}
}

// Compile-time assertion: Client implements council.LLMClient.
var _ council.LLMClient = (*Client)(nil)

// Complete sends a chat completion request to OpenRouter.
// Stub — returns empty response until the HTTP layer is implemented.
func (c *Client) Complete(_ context.Context, _ council.CompletionRequest) (council.CompletionResponse, error) {
	return council.CompletionResponse{}, nil
}
