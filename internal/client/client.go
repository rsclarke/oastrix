// Package client provides an HTTP client for the oastrix API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rsclarke/oastrix/internal/types"
)

// HTTPClient is an interface for HTTP clients that can execute requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is an HTTP client for interacting with the oastrix API.
type Client struct {
	BaseURL    string
	APIKey     string
	httpClient HTTPClient
}

// NewClient creates a new API client with the given base URL and API key.
func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Option is a functional option for configuring the client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client for the API client.
func WithHTTPClient(httpClient HTTPClient) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// CreateToken creates a new token with the given label.
func (c *Client) CreateToken(ctx context.Context, label string) (*types.CreateTokenResponse, error) {
	reqBody := types.CreateTokenRequest{Label: label}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/tokens", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result types.CreateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// GetInteractions retrieves all interactions for the specified token.
func (c *Client) GetInteractions(ctx context.Context, token string) (*types.GetInteractionsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/v1/tokens/"+token+"/interactions", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result types.GetInteractionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// ListTokens retrieves all tokens associated with the API key.
func (c *Client) ListTokens(ctx context.Context) (*types.ListTokensResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/v1/tokens", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result types.ListTokensResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// DeleteToken removes the specified token.
func (c *Client) DeleteToken(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.BaseURL+"/v1/tokens/"+token, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}

	return nil
}

func parseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read error response (status %d): %w", resp.StatusCode, err)
	}

	var errResp types.ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil || errResp.Error == "" {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}
	return errors.New(errResp.Error)
}
