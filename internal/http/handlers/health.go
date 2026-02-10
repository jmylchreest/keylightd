package handlers

import (
	"context"
)

// --- Health Check ---

// HealthInput is the input for health check endpoints.
type HealthInput struct{}

// HealthOutput is the output for health check endpoints.
type HealthOutput struct {
	Body struct {
		Status string `json:"status" doc:"Service health status"`
	}
}

// HealthCheck returns the service health status.
// This is a public endpoint (no auth required).
func HealthCheck(_ context.Context, _ *HealthInput) (*HealthOutput, error) {
	out := &HealthOutput{}
	out.Body.Status = "ok"
	return out, nil
}

// --- Version ---

// VersionInput is the input for the version endpoint.
type VersionInput struct{}

// VersionOutput is the output for the version endpoint.
type VersionOutput struct {
	Body struct {
		Version   string `json:"version" doc:"Semantic version string"`
		Commit    string `json:"commit" doc:"Git commit SHA"`
		BuildDate string `json:"build_date" doc:"Build timestamp (ISO 8601 UTC)"`
	}
}

// NewVersionCheck returns a version handler that reports the given build info.
func NewVersionCheck(version, commit, buildDate string) func(context.Context, *VersionInput) (*VersionOutput, error) {
	return func(_ context.Context, _ *VersionInput) (*VersionOutput, error) {
		out := &VersionOutput{}
		out.Body.Version = version
		out.Body.Commit = commit
		out.Body.BuildDate = buildDate
		return out, nil
	}
}
