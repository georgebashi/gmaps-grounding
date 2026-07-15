# gmaps-grounding

CLI for the [Google Maps Grounding Lite API](https://developers.google.com/maps/ai/grounding-lite) —
Google's geospatial grounding service for AI applications. Talks directly to
its MCP server (`mapstools.googleapis.com/mcp`) over the streamable-HTTP
transport, exposing each grounding tool as a subcommand: places, weather,
routes, name resolution, and Maps URL resolution.

## Install

```sh
go install github.com/georgebashi/gmaps-grounding/cmd/gmaps@latest
```

The installed command is `gmaps`.

## Usage

```sh
export GOOGLE_MAPS_API_KEY=AIza...   # or pass --key

gmaps places coffee near the Ferry Building, SF
gmaps places ramen --near 35.6595,139.7005 --radius 2000
gmaps weather San Francisco, CA --imperial
gmaps weather Tokyo --date 2026-07-18 --hour 9
gmaps route "Ferry Building, SF" "Golden Gate Bridge"
gmaps route 37.7749,-122.4194 "Sausalito, CA" --walk
gmaps resolve "Eiffel Tower, Paris"
gmaps url https://maps.app.goo.gl/abc123
```

Locations are addresses, place names, or `lat,lng` pairs — anywhere one is
accepted. Output is the tool's result pretty-printed (pipes nicely into
`jq`); `--json` prints the raw MCP payload instead. `--endpoint` overrides
the server URL.

Each subcommand wraps one Grounding Lite tool: `places` → `search_places`,
`weather` → `lookup_weather`, `route` → `compute_routes`, `resolve` →
`resolve_names`, `url` → `resolve_maps_urls`. A per-tool
"The caller does not have permission" error means that capability's API
isn't enabled for your key's Google Cloud project.
