# mapsmcp

CLI for the [Google Maps Grounding Lite](https://developers.google.com/maps/ai/grounding-lite)
MCP server (`mapstools.googleapis.com/mcp`): places, weather, and routes
from the command line. Speaks the MCP streamable-HTTP transport directly.

## Install

```sh
go install github.com/georgebashi/mapsmcp@latest
```

## Usage

```sh
export GOOGLE_MAPS_API_KEY=AIza...   # or pass --key

mapsmcp places coffee near the Ferry Building, SF
mapsmcp places ramen --near 35.6595,139.7005 --radius 2000
mapsmcp weather San Francisco, CA --imperial
mapsmcp weather Tokyo --date 2026-07-18 --hour 9
mapsmcp route "Ferry Building, SF" "Golden Gate Bridge"
mapsmcp route 37.7749,-122.4194 "Sausalito, CA" --walk
mapsmcp resolve "Eiffel Tower, Paris"
mapsmcp url https://maps.app.goo.gl/abc123
```

Locations are addresses, place names, or `lat,lng` pairs — anywhere one is
accepted. Output is the tool's result pretty-printed (pipes nicely into
`jq`); `--json` prints the raw MCP payload instead. `--endpoint` overrides
the server URL.

Each subcommand wraps one server tool: `places` → `search_places`,
`weather` → `lookup_weather`, `route` → `compute_routes`, `resolve` →
`resolve_names`, `url` → `resolve_maps_urls`. A per-tool
"The caller does not have permission" error means that capability's API
isn't enabled for your key's Google Cloud project.
