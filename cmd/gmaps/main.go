// gmaps is a CLI for the Google Maps Grounding Lite API's MCP
// server (https://mapstools.googleapis.com/mcp), exposing each server tool
// as a subcommand: places, weather, route, resolve, url.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const version = "0.2.0"

var (
	flagKey      string
	flagEndpoint string
	flagJSON     bool
)

func newClient() *client {
	return &client{endpoint: flagEndpoint, apiKey: flagKey, http: http.DefaultClient}
}

// output prints a tool result: the text content by default, the raw JSON
// payload with --json. Tool-level errors go to stderr with exit code 1.
func output(cmd *cobra.Command, res *toolResult) error {
	if res.IsError {
		msg := "tool call failed"
		for _, c := range res.Content {
			if c.Text != "" {
				msg = c.Text
				break
			}
		}
		return fmt.Errorf("%s", msg)
	}
	if flagJSON {
		var buf bytes.Buffer
		if err := json.Indent(&buf, res.raw, "", "  "); err != nil {
			cmd.OutOrStdout().Write(res.raw)
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), buf.String())
		return nil
	}
	for _, c := range res.Content {
		if c.Text == "" {
			continue
		}
		// Tool text is often a JSON document; pretty-print it if so.
		var buf bytes.Buffer
		if json.Valid([]byte(c.Text)) && json.Indent(&buf, []byte(c.Text), "", "  ") == nil {
			fmt.Fprintln(cmd.OutOrStdout(), buf.String())
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(c.Text))
		}
	}
	return nil
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "gmaps",
		Short: "CLI for the Google Maps Grounding Lite API",
		Long: "gmaps wraps the Google Maps Grounding Lite API — Google's\n" +
			"geospatial grounding service for AI applications — by talking directly\n" +
			"to its MCP server (mapstools.googleapis.com/mcp).\n\n" +
			"It exposes each grounding tool as a subcommand: place search, weather,\n" +
			"routes, name resolution, and Maps URL resolution.\n\n" +
			"Docs: https://developers.google.com/maps/ai/grounding-lite",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if flagKey == "" {
				flagKey = os.Getenv("GOOGLE_MAPS_API_KEY")
			}
			if flagKey == "" {
				return fmt.Errorf("no API key: set $GOOGLE_MAPS_API_KEY or pass --key")
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&flagKey, "key", "", "Google Maps API key (default $GOOGLE_MAPS_API_KEY)")
	root.PersistentFlags().StringVar(&flagEndpoint, "endpoint", defaultEndpoint, "MCP endpoint URL")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "print the raw JSON result")

	root.AddCommand(newPlacesCmd(), newWeatherCmd(), newRouteCmd(), newResolveCmd(), newURLCmd())
	return root
}

func newPlacesCmd() *cobra.Command {
	var near string
	var radius float64
	var lang, region string
	cmd := &cobra.Command{
		Use:   "places <query>...",
		Short: "Search for places, businesses, and addresses",
		Example: `  gmaps places coffee near the Ferry Building, SF
  gmaps places ramen --near 35.6595,139.7005 --radius 2000
  gmaps places "Louvre opening hours" --lang fr`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := map[string]any{"text_query": strings.Join(args, " ")}
			if near != "" {
				center, ok := parseLatLng(near)
				if !ok {
					return fmt.Errorf("--near must be \"lat,lng\", got %q", near)
				}
				circle := map[string]any{"center": center}
				if radius > 0 {
					circle["radiusMeters"] = radius
				}
				req["location_bias"] = map[string]any{"circle": circle}
			} else if radius > 0 {
				return fmt.Errorf("--radius requires --near")
			}
			if lang != "" {
				req["language_code"] = lang
			}
			if region != "" {
				req["region_code"] = region
			}
			res, err := newClient().callTool("search_places", req)
			if err != nil {
				return err
			}
			return output(cmd, res)
		},
	}
	cmd.Flags().StringVar(&near, "near", "", "bias results toward \"lat,lng\"")
	cmd.Flags().Float64Var(&radius, "radius", 0, "bias radius in meters (with --near)")
	cmd.Flags().StringVar(&lang, "lang", "", "result language, e.g. en, ja, en_US")
	cmd.Flags().StringVar(&region, "region", "", "region code for place details, e.g. US")
	return cmd
}

func newWeatherCmd() *cobra.Command {
	var date string
	var hour int
	var imperial bool
	cmd := &cobra.Command{
		Use:   "weather <location>...",
		Short: "Current weather or forecast for a location",
		Long: "Current weather or forecast for a location.\n\n" +
			"The location is an address, place name, or \"lat,lng\" pair.\n" +
			"Forecasts reach 10 days ahead (120 hours for hourly); history reaches 24 hours back.",
		Example: `  gmaps weather San Francisco, CA
  gmaps weather 37.7749,-122.4194 --imperial
  gmaps weather Tokyo --date 2026-07-18 --hour 9`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := map[string]any{"location": location(strings.Join(args, " "))}
			if date != "" {
				var y, m, d int
				if _, err := fmt.Sscanf(date, "%d-%d-%d", &y, &m, &d); err != nil {
					return fmt.Errorf("--date must be YYYY-MM-DD, got %q", date)
				}
				req["date"] = map[string]any{"year": y, "month": m, "day": d}
			}
			if cmd.Flags().Changed("hour") {
				req["hour"] = hour
			}
			if imperial {
				req["unitsSystem"] = "IMPERIAL"
			}
			res, err := newClient().callTool("lookup_weather", req)
			if err != nil {
				return err
			}
			return output(cmd, res)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "forecast date (YYYY-MM-DD, local to the location)")
	cmd.Flags().IntVar(&hour, "hour", 0, "hour of day (0-23, local to the location)")
	cmd.Flags().BoolVar(&imperial, "imperial", false, "use imperial units (default metric)")
	return cmd
}

func newRouteCmd() *cobra.Command {
	var walk bool
	cmd := &cobra.Command{
		Use:   "route <origin> <destination>",
		Short: "Compute a route between two places",
		Long: "Compute a route between two places.\n\n" +
			"Origin and destination are addresses, place names, or \"lat,lng\" pairs.",
		Example: `  gmaps route "Ferry Building, SF" "Golden Gate Bridge"
  gmaps route 37.7749,-122.4194 "Sausalito, CA" --walk`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := map[string]any{
				"origin":      location(args[0]),
				"destination": location(args[1]),
			}
			if walk {
				req["travelMode"] = "WALK"
			}
			res, err := newClient().callTool("compute_routes", req)
			if err != nil {
				return err
			}
			return output(cmd, res)
		},
	}
	cmd.Flags().BoolVar(&walk, "walk", false, "walking directions (default driving)")
	return cmd
}

func newResolveCmd() *cobra.Command {
	var region string
	cmd := &cobra.Command{
		Use:   "resolve <place>...",
		Short: "Resolve place names or addresses to place IDs and coordinates",
		Long: "Resolve place names or addresses to place IDs and coordinates.\n\n" +
			"Each argument is one query (quote multi-word names); up to 20 per call.",
		Example: `  gmaps resolve "Eiffel Tower, Paris"
  gmaps resolve "Ferry Building, SF" "Golden Gate Bridge" --region US`,
		Args: cobra.RangeArgs(1, 20),
		RunE: func(cmd *cobra.Command, args []string) error {
			queries := make([]map[string]any, len(args))
			for i, q := range args {
				queries[i] = map[string]any{"text": q}
			}
			req := map[string]any{"queries": queries}
			if region != "" {
				req["regionCode"] = region
			}
			res, err := newClient().callTool("resolve_names", req)
			if err != nil {
				return err
			}
			return output(cmd, res)
		},
	}
	cmd.Flags().StringVar(&region, "region", "", "region code to bias results, e.g. US")
	return cmd
}

func newURLCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "url <maps-url>...",
		Short: "Resolve Google Maps URLs to place details",
		Long: "Resolve Google Maps URLs to place details.\n\n" +
			"Accepts maps.app.goo.gl short links and google.com/maps place URLs\n" +
			"pointing at a single place; up to 20 per call.",
		Example: `  gmaps url https://maps.app.goo.gl/abc123`,
		Args:    cobra.RangeArgs(1, 20),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := newClient().callTool("resolve_maps_urls", map[string]any{"urls": args})
			if err != nil {
				return err
			}
			return output(cmd, res)
		},
	}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gmaps:", err)
		os.Exit(1)
	}
}
