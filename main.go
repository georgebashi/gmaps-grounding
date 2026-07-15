// mapsmcp is a small CLI client for the Google Maps Grounding Lite MCP
// server (https://mapstools.googleapis.com/mcp). It speaks the MCP
// streamable-HTTP transport directly with no dependencies.
//
// Usage:
//
//	mapsmcp tools                        list available tools
//	mapsmcp call <tool> ['{json args}']  call a tool
//
// The API key is read from -key or $GOOGLE_MAPS_API_KEY.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
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

// call sends one JSON-RPC request and returns its result, handling both
// plain-JSON and SSE response bodies.
func (c *client) call(method string, params any) (json.RawMessage, error) {
	c.nextID++
	id := c.nextID
	resp, err := c.post(rpcRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%s: HTTP %d: %s", method, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	payload, err := extractPayload(resp.Header.Get("Content-Type"), resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	var r rpcResponse
	if err := json.Unmarshal(payload, &r); err != nil {
		return nil, fmt.Errorf("%s: bad response: %w", method, err)
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

func (c *client) initialize() error {
	_, err := c.call("initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "mapsmcp", "version": "0.1.0"},
	})
	if err != nil {
		return err
	}
	return c.notify("notifications/initialized")
}

func printJSON(raw json.RawMessage) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		os.Stdout.Write(raw)
		fmt.Println()
		return
	}
	fmt.Println(buf.String())
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: mapsmcp [flags] <command>

commands:
  tools                        list available tools
  call <tool> ['{json args}']  call a tool with JSON arguments

flags:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
The API key comes from -key or $GOOGLE_MAPS_API_KEY.

examples:
  mapsmcp tools
  mapsmcp call search_places '{"text_query": "coffee near Ferry Building SF"}'
`)
}

func main() {
	key := flag.String("key", os.Getenv("GOOGLE_MAPS_API_KEY"), "Google Maps API key (default $GOOGLE_MAPS_API_KEY)")
	endpoint := flag.String("url", defaultEndpoint, "MCP endpoint URL")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	if *key == "" {
		fmt.Fprintln(os.Stderr, "mapsmcp: no API key: set $GOOGLE_MAPS_API_KEY or pass -key")
		os.Exit(1)
	}

	c := &client{endpoint: *endpoint, apiKey: *key, http: http.DefaultClient}

	run := func() error {
		switch args[0] {
		case "tools":
			if err := c.initialize(); err != nil {
				return err
			}
			result, err := c.call("tools/list", map[string]any{})
			if err != nil {
				return err
			}
			printJSON(result)
		case "call":
			if len(args) < 2 {
				return fmt.Errorf("usage: mapsmcp call <tool> ['{json args}']")
			}
			toolArgs := map[string]any{}
			if len(args) > 2 {
				if err := json.Unmarshal([]byte(args[2]), &toolArgs); err != nil {
					return fmt.Errorf("bad JSON arguments: %w", err)
				}
			}
			if err := c.initialize(); err != nil {
				return err
			}
			result, err := c.call("tools/call", map[string]any{
				"name":      args[1],
				"arguments": toolArgs,
			})
			if err != nil {
				return err
			}
			printJSON(result)
		default:
			return fmt.Errorf("unknown command %q", args[0])
		}
		return nil
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mapsmcp:", err)
		os.Exit(1)
	}
}
