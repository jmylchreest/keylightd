// Package mw provides middleware and registration helpers for the keylightd HTTP API.
package mw

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// SecurityScheme is the name of the security scheme used in OpenAPI.
const SecurityScheme = "apiKeyAuth"

// OperationOption is a function that modifies a Huma operation.
type OperationOption func(*huma.Operation)

// WithTags adds tags to the operation.
func WithTags(tags ...string) OperationOption {
	return func(op *huma.Operation) {
		op.Tags = append(op.Tags, tags...)
	}
}

// WithSummary sets the operation summary.
func WithSummary(summary string) OperationOption {
	return func(op *huma.Operation) {
		op.Summary = summary
	}
}

// WithDescription sets the operation description.
func WithDescription(desc string) OperationOption {
	return func(op *huma.Operation) {
		op.Description = desc
	}
}

// WithOperationID sets a custom operation ID.
func WithOperationID(id string) OperationOption {
	return func(op *huma.Operation) {
		op.OperationID = id
	}
}

// WithHidden hides the operation from OpenAPI documentation.
func WithHidden() OperationOption {
	return func(op *huma.Operation) {
		op.Hidden = true
	}
}

// WithDefaultStatus sets the default HTTP status code for successful responses.
func WithDefaultStatus(status int) OperationOption {
	return func(op *huma.Operation) {
		op.DefaultStatus = status
	}
}

// ProtectedGet registers a GET endpoint that requires API key auth.
func ProtectedGet[I, O any](api huma.API, path string, handler func(ctx context.Context, input *I) (*O, error), opts ...OperationOption) {
	op := huma.Operation{
		Method:   http.MethodGet,
		Path:     path,
		Security: []map[string][]string{{SecurityScheme: {}}},
	}
	for _, opt := range opts {
		opt(&op)
	}
	huma.Register(api, op, handler)
}

// ProtectedPost registers a POST endpoint that requires API key auth.
func ProtectedPost[I, O any](api huma.API, path string, handler func(ctx context.Context, input *I) (*O, error), opts ...OperationOption) {
	op := huma.Operation{
		Method:   http.MethodPost,
		Path:     path,
		Security: []map[string][]string{{SecurityScheme: {}}},
	}
	for _, opt := range opts {
		opt(&op)
	}
	huma.Register(api, op, handler)
}

// ProtectedPut registers a PUT endpoint that requires API key auth.
func ProtectedPut[I, O any](api huma.API, path string, handler func(ctx context.Context, input *I) (*O, error), opts ...OperationOption) {
	op := huma.Operation{
		Method:   http.MethodPut,
		Path:     path,
		Security: []map[string][]string{{SecurityScheme: {}}},
	}
	for _, opt := range opts {
		opt(&op)
	}
	huma.Register(api, op, handler)
}

// ProtectedDelete registers a DELETE endpoint that requires API key auth.
func ProtectedDelete[I, O any](api huma.API, path string, handler func(ctx context.Context, input *I) (*O, error), opts ...OperationOption) {
	op := huma.Operation{
		Method:   http.MethodDelete,
		Path:     path,
		Security: []map[string][]string{{SecurityScheme: {}}},
	}
	for _, opt := range opts {
		opt(&op)
	}
	huma.Register(api, op, handler)
}
