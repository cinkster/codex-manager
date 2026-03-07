package htmlbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultBaseURL = "https://api.htmlbucket.com"

// Client uploads rendered HTML to htmlbucket.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an htmlbucket client with default API base URL.
func NewClient(apiKey string) *Client {
	return NewClientWithBaseURL(apiKey, defaultBaseURL, nil)
}

// NewClientWithBaseURL creates an htmlbucket client with a custom base URL.
func NewClientWithBaseURL(apiKey string, baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		apiKey:     strings.TrimSpace(apiKey),
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: httpClient,
	}
}

// Upload publishes HTML and returns the public htmlbucket URL.
func (c *Client) Upload(ctx context.Context, html string) (string, error) {
	if c == nil {
		return "", errors.New("client is nil")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return "", errors.New("api key is empty")
	}
	if strings.TrimSpace(c.baseURL) == "" {
		return "", errors.New("base URL is empty")
	}
	if html == "" {
		return "", errors.New("content is empty")
	}

	payload, err := json.Marshal(map[string]string{"content": html})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/upload", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", err
	}
	bodyText := strings.TrimSpace(string(bodyBytes))
	if resp.StatusCode != http.StatusOK {
		if bodyText != "" {
			return "", fmt.Errorf("status %d: %s", resp.StatusCode, bodyText)
		}
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var parsed struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return "", fmt.Errorf("invalid success response: %w", err)
	}
	parsed.URL = strings.TrimSpace(parsed.URL)
	if parsed.URL == "" {
		return "", errors.New("success response missing url")
	}
	return parsed.URL, nil
}
