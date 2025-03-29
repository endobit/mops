package mops

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
)

// Client is a minimalist REST client for making HTTP requests to the mops service.
type Client struct {
	http.Client

	URL   string
	Debug bool
}

// Response represents an HTTP response from the mops service.
type Response struct {
	Header http.Header
	Body   io.Reader
}

// Post sends a POST request to the specified URL with the given request body
// and decodes the response into the provided struct.
func (c *Client) Post(ctx context.Context, url string, req, resp any) error {
	return c.request(ctx, http.MethodPost, url, req, resp)
}

// Put sends a PUT request to the specified URL with the given request body
// and decodes the response into the provided struct.
func (c *Client) Put(ctx context.Context, url string, req, resp any) error {
	return c.request(ctx, http.MethodPut, url, req, resp)
}

// Patch sends a PATCH request to the specified URL with the given request body
// and decodes the response into the provided struct.
func (c *Client) Patch(ctx context.Context, url string, req, resp any) error {
	return c.request(ctx, http.MethodPatch, url, req, resp)
}

// Get sends a GET request to the specified URL and decodes the response into
// the provided struct.
func (c *Client) Get(ctx context.Context, url string, resp any) error {
	return c.request(ctx, http.MethodGet, url, nil, resp)
}

// Delete sends a DELETE request to the specified URL.
func (c *Client) Delete(ctx context.Context, url string) error {
	return c.request(ctx, http.MethodDelete, url, nil, nil)
}

func (c *Client) request(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader = http.NoBody

	if in != nil { // encode body
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}

		body = bytes.NewReader(b)
	}

	resp, err := c.Request(ctx, method, path, nil, body)
	if err != nil {
		return err
	}

	if out != nil { // parse response
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
	}

	return nil
}

// Request sends an HTTP request to the specified path with the given method,
// headers, and body. It returns the HTTP response or an error if the request
// fails.
func (c *Client) Request(ctx context.Context, method, path string, header map[string]string, body io.Reader) (*Response, error) {
	if body == nil {
		body = http.NoBody
	}

	req, err := http.NewRequestWithContext(ctx, method, c.url(path), body)
	if err != nil {
		return nil, err
	}

	for k, v := range header {
		req.Header.Set(k, v)
	}

	if c.Debug {
		b, err := httputil.DumpRequest(req, true)
		if err != nil {
			return nil, err
		}

		fmt.Printf("REQUEST:\n%s", hex.Dump(b))
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if c.Debug {
		b, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return nil, err
		}

		fmt.Printf("RESPONSE:\n%s", hex.Dump(b))
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		return nil, fmt.Errorf("http error: %s", resp.Status)
	}

	var buf bytes.Buffer

	if _, err = buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}

	return &Response{Header: resp.Header.Clone(), Body: &buf}, nil
}

// url returns the url for the given path.
func (c *Client) url(path string) string {
	path = strings.TrimPrefix(path, "/") // prevent accidental double slash

	return c.URL + "/" + path
}
