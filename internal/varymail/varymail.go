// Package varymail Wraps vary.email fetch API:
// Queries services/stock, purchases mailbox, fetches latest verification code by purchase ID.
package varymail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL vary.email API base URL.
const DefaultBaseURL = "https://vary.email"

// DefaultServiceName Fixed service name (receives ChatGPT/OpenAI verification code).
const DefaultServiceName = "chatgpt"

// Known business errors.
var (
	ErrUnauthorized = errors.New("varymail: API Key is missing or invalid")
	ErrNoBalance    = errors.New("varymail: Insufficient balance, please top up")
	ErrOutOfStock   = errors.New("varymail: No mailboxes currently available for this service")
	ErrPickup       = errors.New("varymail: Pickup failed, mailbox may have expired")
	ErrNoService    = errors.New("varymail: chatgpt service not found in store")
)

// Client vary.email API client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New Creates client. Uses default URL if baseURL is empty.
func New(baseURL, apiKey string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Service On-sale service and stock.
type Service struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Logo      string `json:"logo"`
	Stock     string `json:"stock"`     // ok / low / out
	Available int    `json:"available"` // Available mailbox count
}

// Purchase Mailbox purchase result.
type Purchase struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	Service   string `json:"service"`
	Status    string `json:"status"` // active / expired
	CreatedAt string `json:"created_at"`
}

// CodeMsg Latest fetched email message.
type CodeMsg struct {
	ID         string `json:"id"`
	From       string `json:"from"`
	Code       string `json:"code"`
	ReceivedAt string `json:"received_at"`
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// do Executes request and unmarshals data.
func (c *Client) do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	if c.apiKey == "" {
		return nil, ErrUnauthorized
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if e := mapStatus(resp.StatusCode); e != nil {
		return nil, e
	}

	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("varymail: failed to parse response (HTTP %d)", resp.StatusCode)
	}
	if env.Code != 200 {
		if e := mapStatus(env.Code); e != nil {
			return nil, e
		}
		msg := strings.TrimSpace(env.Message)
		if msg == "" {
			msg = fmt.Sprintf("code=%d", env.Code)
		}
		return nil, fmt.Errorf("varymail: %s", msg)
	}
	return env.Data, nil
}

func mapStatus(code int) error {
	switch code {
	case 401:
		return ErrUnauthorized
	case 402:
		return ErrNoBalance
	case 409:
		return ErrOutOfStock
	case 502:
		return ErrPickup
	}
	return nil
}

// Services Fetches list of services and price.
func (c *Client) Services(ctx context.Context) (items []Service, price float64, err error) {
	data, err := c.do(ctx, http.MethodGet, "/api/shop/services", nil)
	if err != nil {
		return nil, 0, err
	}
	var out struct {
		Items []Service `json:"items"`
		Price float64   `json:"price"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, 0, fmt.Errorf("varymail: failed to parse service list")
	}
	return out.Items, out.Price, nil
}

// ServiceByName Finds service by name (case-insensitive).
func (c *Client) ServiceByName(ctx context.Context, name string) (svc Service, price float64, err error) {
	items, price, err := c.Services(ctx)
	if err != nil {
		return Service{}, 0, err
	}
	name = strings.ToLower(strings.TrimSpace(name))
	for _, s := range items {
		if strings.ToLower(strings.TrimSpace(s.Name)) == name {
			return s, price, nil
		}
	}
	for _, s := range items {
		if strings.Contains(strings.ToLower(s.Name), name) {
			return s, price, nil
		}
	}
	return Service{}, price, ErrNoService
}

// Buy Purchases mailbox by service ID.
func (c *Client) Buy(ctx context.Context, serviceID int) (Purchase, float64, error) {
	data, err := c.do(ctx, http.MethodPost, "/api/my/purchases", map[string]any{"service_id": serviceID})
	if err != nil {
		return Purchase{}, 0, err
	}
	var out struct {
		Item    Purchase `json:"item"`
		Balance float64  `json:"balance"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return Purchase{}, 0, fmt.Errorf("varymail: failed to parse order response")
	}
	return out.Item, out.Balance, nil
}

// Code Fetches latest email and verification code by purchase ID.
func (c *Client) Code(ctx context.Context, purchaseID int) (msg CodeMsg, hasMail bool, err error) {
	data, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/my/purchases/%d/code", purchaseID), nil)
	if err != nil {
		return CodeMsg{}, false, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return CodeMsg{}, false, nil
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return CodeMsg{}, false, fmt.Errorf("varymail: failed to parse code response")
	}
	return msg, true, nil
}
