package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const defaultEndpoint = "https://mapstools.googleapis.com/mcp"

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// toolResult is the MCP tools/call result shape.
type toolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`

	raw json.RawMessage // full result payload, for --json output
}

type client struct {
	endpoint  string
	apiKey    string
	sessionID string
	nextID    int
	http      *http.Client
}

func (c *client) post(body any) (*http.Response, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}
	return resp, nil
}

// rpc sends one JSON-RPC request and returns its result, handling both
// plain-JSON and SSE response bodies. The server reports tool-level
// failures as HTTP 400 with a JSON-RPC body, so non-200 statuses with a
// parseable body are surfaced through the normal result path.
func (c *client) rpc(method string, params any) (json.RawMessage, error) {
	c.nextID++
	id := c.nextID
	resp, err := c.post(rpcRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := extractPayload(resp.Header.Get("Content-Type"), resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	var r rpcResponse
	if jsonErr := json.Unmarshal(payload, &r); jsonErr != nil || (r.Result == nil && r.Error == nil) {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: HTTP %d: %s", method, resp.StatusCode, truncate(string(payload), 500))
		}
		return nil, fmt.Errorf("%s: bad response: %s", method, truncate(string(payload), 500))
	}
	if r.Error != nil {
		return nil, fmt.Errorf("%s: server error %d: %s", method, r.Error.Code, r.Error.Message)
	}
	return r.Result, nil
}

// notify sends a JSON-RPC notification (no id, no response expected).
func (c *client) notify(method string) error {
	resp, err := c.post(rpcRequest{JSONRPC: "2.0", Method: method})
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

func (c *client) initialize() error {
	_, err := c.rpc("initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "mapsmcp", "version": version},
	})
	if err != nil {
		return err
	}
	return c.notify("notifications/initialized")
}

// callTool runs the MCP handshake plus one tools/call.
func (c *client) callTool(name string, args map[string]any) (*toolResult, error) {
	if err := c.initialize(); err != nil {
		return nil, err
	}
	raw, err := c.rpc("tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return nil, err
	}
	var res toolResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("bad tool result: %w", err)
	}
	res.raw = raw
	return &res, nil
}

// extractPayload returns the JSON-RPC response body, decoding SSE framing
// (the streamable-HTTP transport may answer either way).
func extractPayload(contentType string, body io.Reader) ([]byte, error) {
	if !strings.HasPrefix(contentType, "text/event-stream") {
		return io.ReadAll(body)
	}
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var data []string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		case line == "" && len(data) > 0:
			// End of the first complete event: the response message.
			return []byte(strings.Join(data, "\n")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(data) > 0 {
		return []byte(strings.Join(data, "\n")), nil
	}
	return nil, fmt.Errorf("empty SSE stream")
}

// parseLatLng parses "lat,lng" into a LatLng object, or returns ok=false
// if the string is not a coordinate pair.
func parseLatLng(s string) (map[string]any, bool) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return nil, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return nil, false
	}
	return map[string]any{"latitude": lat, "longitude": lng}, true
}

// location builds a Location/Waypoint object: "lat,lng" becomes latLng,
// anything else is treated as a human-readable address or place name.
func location(s string) map[string]any {
	if ll, ok := parseLatLng(s); ok {
		return map[string]any{"latLng": ll}
	}
	return map[string]any{"address": s}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
