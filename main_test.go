package main

import (
	"strings"
	"testing"
)

func TestExtractPayloadJSON(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`
	got, err := extractPayload("application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("got %q, want %q", got, body)
	}
}

func TestExtractPayloadSSE(t *testing.T) {
	stream := "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\n" +
		"data: \"result\":{}}\n\n" +
		"data: {\"ignored\":\"second event\"}\n\n"
	got, err := extractPayload("text/event-stream; charset=utf-8", strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"jsonrpc\":\"2.0\",\"id\":1,\n\"result\":{}}"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractPayloadSSENoTrailingBlank(t *testing.T) {
	stream := "data: {\"a\":1}"
	got, err := extractPayload("text/event-stream", strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"a":1}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractPayloadEmptySSE(t *testing.T) {
	if _, err := extractPayload("text/event-stream", strings.NewReader("")); err == nil {
		t.Error("want error for empty stream")
	}
}
