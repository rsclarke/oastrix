package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	BaseURL    string
	APIKey     string
	httpClient HTTPClient
}

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

type Option func(*Client)

func WithHTTPClient(httpClient HTTPClient) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

type CreateTokenRequest struct {
	Label string `json:"label,omitempty"`
}

type CreateTokenResponse struct {
	Token    string            `json:"token"`
	Payloads map[string]string `json:"payloads"`
}

type InteractionResponse struct {
	ID         int64                  `json:"id"`
	Kind       string                 `json:"kind"`
	OccurredAt string                 `json:"occurred_at"`
	RemoteIP   string                 `json:"remote_ip"`
	RemotePort int                    `json:"remote_port"`
	TLS        bool                   `json:"tls"`
	Summary    string                 `json:"summary"`
	HTTP       *HTTPInteractionDetail `json:"http,omitempty"`
	DNS        *DNSInteractionDetail  `json:"dns,omitempty"`
}

type HTTPInteractionDetail struct {
	Method  string              `json:"method"`
	Scheme  string              `json:"scheme"`
	Host    string              `json:"host"`
	Path    string              `json:"path"`
	Query   string              `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

type DNSInteractionDetail struct {
	QName    string `json:"qname"`
	QType    int    `json:"qtype"`
	QClass   int    `json:"qclass"`
	RD       bool   `json:"rd"`
	Opcode   int    `json:"opcode"`
	DNSID    int    `json:"dns_id"`
	Protocol string `json:"protocol"`
}

type GetInteractionsResponse struct {
	Token        string                `json:"token"`
	Interactions []InteractionResponse `json:"interactions"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type TokenInfo struct {
	Token            string  `json:"token"`
	Label            *string `json:"label"`
	CreatedAt        string  `json:"created_at"`
	InteractionCount int     `json:"interaction_count"`
}

type ListTokensResponse struct {
	Tokens []TokenInfo `json:"tokens"`
}

func (c *Client) CreateToken(label string) (*CreateTokenResponse, error) {
	reqBody := CreateTokenRequest{Label: label}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/v1/tokens", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result CreateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetInteractions(token string) (*GetInteractionsResponse, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/v1/tokens/"+token+"/interactions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result GetInteractionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListTokens() (*ListTokensResponse, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/v1/tokens", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result ListTokensResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteToken(token string) error {
	req, err := http.NewRequest("DELETE", c.BaseURL+"/v1/tokens/"+token, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}

	return nil
}

func parseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil || errResp.Error == "" {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}
	return fmt.Errorf("%s", errResp.Error)
}
