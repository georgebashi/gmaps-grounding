# mapsmcp

Tiny CLI client for the [Google Maps Grounding Lite](https://developers.google.com/maps/ai/grounding-lite)
MCP server (`mapstools.googleapis.com/mcp`). Speaks the MCP streamable-HTTP
transport directly — stdlib only, no dependencies.

## Install

```sh
go install github.com/georgebashi/mapsmcp@latest
```

## Usage

```sh
export GOOGLE_MAPS_API_KEY=AIza...   # or pass -key

mapsmcp tools                        # list available tools
mapsmcp call search_places '{"text_query": "coffee near Ferry Building SF"}'
mapsmcp call lookup_weather '{"location": {"address": "San Francisco, CA"}, "unitsSystem": "IMPERIAL"}'
```

Run `mapsmcp tools` first to see the current tool names and their input
schemas — output is pretty-printed JSON, so it pipes nicely into `jq`.

The `-url` flag overrides the endpoint if Google ever moves it.
