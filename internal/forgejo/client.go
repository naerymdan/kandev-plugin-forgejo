// Package forgejo is the concrete provider adapter layer. It owns every
// Forgejo/Gitea-specific URL, pagination token, authentication detail, and
// error mapping, and exposes only the narrow ports the provider-neutral
// source-control recipe declares.
//
// Forgejo is a hard fork of Gitea and both serve the same versioned REST
// surface at <base>/api/v1. This client deliberately restricts itself to
// endpoints and fields that exist on both, so one plugin serves either host.
package forgejo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxResponseBytes bounds any single API response the plugin will buffer. A
// misconfigured or hostile base_url must not be able to exhaust plugin memory.
const maxResponseBytes = 8 << 20

// ErrNotFound is returned for 404 and 410 responses. Callers translate it into
// "not owned by this connection" rather than surfacing it as a transport error.
var ErrNotFound = errors.New("forgejo: resource not found")

// ErrUnauthorized is returned for 401/403. It never carries the response body,
// which on some deployments echoes the presented token.
var ErrUnauthorized = errors.New("forgejo: not authorized")

// Client is a bounded REST v1 client for one Forgejo or Gitea instance.
type Client struct {
	baseURL *url.URL
	token   string
	http    *http.Client
}

// NewClient validates baseURL and returns a client for the instance it names.
// The token is kept in memory only; it is never logged or echoed.
func NewClient(baseURL, token string, httpClient *http.Client) (*Client, error) {
	normalized, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("forgejo: parse instance URL: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("forgejo: an access token is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{baseURL: parsed, token: strings.TrimSpace(token), http: httpClient}, nil
}

// NormalizeBaseURL canonicalizes an operator-supplied instance URL into the
// value used as the connection scope. It requires an absolute http(s) URL and
// strips any query, fragment, and trailing slash so that the same instance
// always produces the same scope string.
func NormalizeBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("forgejo: instance URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("forgejo: parse instance URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("forgejo: instance URL must be http or https, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", errors.New("forgejo: instance URL must include a host")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	return parsed.String(), nil
}

// Scope is the connection scope for this client: the normalized instance URL.
func (c *Client) Scope() string { return c.baseURL.String() }

// Host is the instance hostname, used as the repository provider host.
func (c *Client) Host() string { return c.baseURL.Host }

// apiV1 is the versioned REST surface Forgejo and Gitea share.
const apiV1 = "/api/v1"

// apiForgejoV1 is Forgejo's own API namespace. Gitea does not serve it, which
// makes it a reliable flavor probe.
const apiForgejoV1 = "/api/forgejo/v1"

// get issues an authenticated GET against /api/v1<path> and decodes JSON into
// out. query may be nil.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, apiV1, path, query, nil, out)
}

// post issues an authenticated POST against /api/v1<path>.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, apiV1, path, nil, body, out)
}

func (c *Client) do(ctx context.Context, method, prefix, path string, query url.Values, body, out any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// `path` arrives already percent-escaped (see pathSegment). url.URL.Path
	// holds the DECODED path and re-escapes '%' on String(), so the escaped
	// form must go in RawPath and the decoded form in Path — otherwise a
	// repository named "a/b" is requested as "a%252Fb".
	endpoint := *c.baseURL
	escapedPath := strings.TrimSuffix(endpoint.EscapedPath(), "/") + prefix + path
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return fmt.Errorf("forgejo: build request path: %w", err)
	}
	endpoint.Path = decodedPath
	endpoint.RawPath = escapedPath
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("forgejo: encode request body: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), payload)
	if err != nil {
		return fmt.Errorf("forgejo: build request: %w", err)
	}
	// Forgejo and Gitea both accept the "token <value>" scheme for personal
	// access tokens on every REST v1 endpoint.
	request.Header.Set("Authorization", "token "+c.token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(request)
	if err != nil {
		// Wrap without the URL's userinfo and without the token header.
		return fmt.Errorf("forgejo: %s %s: %w", method, path, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		_ = response.Body.Close()
	}()

	switch {
	case response.StatusCode == http.StatusNotFound, response.StatusCode == http.StatusGone:
		return ErrNotFound
	case response.StatusCode == http.StatusUnauthorized, response.StatusCode == http.StatusForbidden:
		return ErrUnauthorized
	case response.StatusCode >= 400:
		return fmt.Errorf("forgejo: %s %s: unexpected status %d", method, path, response.StatusCode)
	}

	if out == nil {
		return nil
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("forgejo: decode %s response: %w", path, err)
	}
	return nil
}
