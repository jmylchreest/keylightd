// Package main provides a CLI tool to generate the OpenAPI specification for the keylightd API.
// This binary uses the shared route definitions with stub handlers to produce an accurate
// OpenAPI spec without requiring any real services or dependencies.
//
// Usage:
//
//	go run ./cmd/keylight-openapi > openapi.json
//	go run ./cmd/keylight-openapi -yaml > openapi.yaml
//	go run ./cmd/keylight-openapi -output openapi.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/jmylchreest/keylightd/internal/http/routes"
)

var (
	// version is set via ldflags at build time.
	version = "dev"
)

func main() {
	outputFile := flag.String("output", "", "Output file path (default: stdout)")
	outputYAML := flag.Bool("yaml", false, "Output as YAML instead of JSON")
	baseURL := flag.String("base-url", "", "Base URL for the API server")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// Create a minimal chi router — we won't actually serve requests
	router := chi.NewRouter()

	// Create Huma API with shared config
	cfg := routes.NewHumaConfig(version, *baseURL)
	api := humachi.New(router, cfg)

	// Register all routes with stub handlers
	routes.Register(api, routes.StubHandlers())

	// Get the OpenAPI spec
	spec := api.OpenAPI()

	// Marshal the spec
	var data []byte
	var err error

	if *outputYAML {
		data, err = yaml.Marshal(spec)
	} else {
		data, err = json.MarshalIndent(spec, "", "  ")
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	// Output to file or stdout
	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing to file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "OpenAPI spec written to %s\n", *outputFile)
	} else {
		fmt.Print(string(data))
	}
}
