// Package finimpulse is a client for the Finimpulse market data API. Each
// endpoint lives in its own file and decodes through the shared get helper.
package finimpulse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// statusOK is the API's own success code, sent inside the response body.
const statusOK = 20000

// defaultTimeout bounds a single request so one slow call cannot stall a batch.
const defaultTimeout = 10 * time.Second

// maxErrorBody caps how much of a failed response is kept for the error.
const maxErrorBody = 512

// Client calls the API with a bearer token.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New returns a client for the API at baseURL.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

// Meta is the envelope every response carries around its result.
type Meta struct {
	TaskID        string  `json:"task_id"`
	StatusCode    int     `json:"status_code"`
	StatusMessage string  `json:"status_message"`
	Live          bool    `json:"live"`
	Cost          float64 `json:"cost"`
}

// Response is a decoded API response: the envelope and the endpoint's result.
type Response[T any] struct {
	Meta
	Result T `json:"result"`
}

// APIError is a request the API refused, either by HTTP status or by the
// status code in the body.
type APIError struct {
	Path       string
	HTTPStatus int
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("finimpulse %s: http %d, status %d: %s",
		e.Path, e.HTTPStatus, e.StatusCode, e.Message)
}

// get requests path and decodes the response into Response[T].
func get[T any](ctx context.Context, c *Client, path string) (Response[T], error) {
	return do[T](ctx, c, http.MethodGet, path, nil)
}

// post sends payload to path as JSON and decodes the response into Response[T].
func post[T any](ctx context.Context, c *Client, path string, payload any) (Response[T], error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Response[T]{}, fmt.Errorf("finimpulse %s: failed to encode request: %w", path, err)
	}
	return do[T](ctx, c, http.MethodPost, path, encoded)
}

// do sends one request, with body as JSON when given, and decodes the response.
func do[T any](ctx context.Context, c *Client, method, path string, body []byte) (Response[T], error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return Response[T]{}, fmt.Errorf("finimpulse %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Response[T]{}, fmt.Errorf("finimpulse %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response[T]{}, fmt.Errorf("finimpulse %s: failed to read body: %w", path, err)
	}

	var decoded Response[T]
	decodeErr := json.Unmarshal(respBody, &decoded)
	if resp.StatusCode != http.StatusOK || (decodeErr == nil && decoded.StatusCode != statusOK) {
		return Response[T]{}, apiError(path, resp.StatusCode, decoded.Meta, respBody)
	}
	if decodeErr != nil {
		return Response[T]{}, fmt.Errorf("finimpulse %s: failed to decode body: %w", path, decodeErr)
	}
	return decoded, nil
}

// apiError describes a refused request, preferring the API's own message and
// falling back to the start of the raw body.
func apiError(path string, httpStatus int, meta Meta, body []byte) *APIError {
	message := meta.StatusMessage
	if message == "" {
		message = string(body[:min(len(body), maxErrorBody)])
	}
	return &APIError{
		Path: path, HTTPStatus: httpStatus, StatusCode: meta.StatusCode, Message: message,
	}
}
