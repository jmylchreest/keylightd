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
