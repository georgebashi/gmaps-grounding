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

func TestParseLatLng(t *testing.T) {
	ll, ok := parseLatLng("37.7749, -122.4194")
	if !ok || ll["latitude"] != 37.7749 || ll["longitude"] != -122.4194 {
		t.Errorf("got %v ok=%v", ll, ok)
	}
	for _, bad := range []string{"San Francisco, CA", "1,2,3", "91,0", "0,181", "a,b", "Main St, Springfield"} {
		if _, ok := parseLatLng(bad); ok {
			t.Errorf("parseLatLng(%q) unexpectedly ok", bad)
		}
	}
}

func TestLocation(t *testing.T) {
	if loc := location("35.6,139.7"); loc["latLng"] == nil {
		t.Errorf("coordinate pair not parsed as latLng: %v", loc)
	}
	if loc := location("Eiffel Tower, Paris"); loc["address"] != "Eiffel Tower, Paris" {
		t.Errorf("address not preserved: %v", loc)
	}
}
