// Package strudel provides a client for communicating with a local Strudel REPL server.
package strudel

import "net/http"

// Client talks to a running Strudel server instance.
type Client struct {
	BaseURL    string
	httpClient *http.Client
}

// New returns a Client pointed at the given Strudel server base URL.
func New(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		httpClient: &http.Client{},
	}
}
