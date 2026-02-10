package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jmylchreest/keylightd/internal/apikey"
	kerrors "github.com/jmylchreest/keylightd/internal/errors"
)

// --- Create API Key ---

// CreateAPIKeyInput is the input for creating a new API key.
type CreateAPIKeyInput struct {
	Body struct {
		Name      string `json:"name" doc:"Display name for the API key" minLength:"1"`
		ExpiresIn string `json:"expires_in,omitempty" doc:"Duration string (e.g., '720h', '30d')"`
	}
}

// CreateAPIKeyOutput is the output for creating a new API key (HTTP 201).
type CreateAPIKeyOutput struct {
	Body APIKeyResponse
}

// --- List API Keys ---

// ListAPIKeysInput is the input for listing all API keys.
type ListAPIKeysInput struct{}

// ListAPIKeysOutput is the output for listing all API keys.
type ListAPIKeysOutput struct {
	Body []APIKeyResponse
}

// --- Delete API Key ---

// DeleteAPIKeyInput is the input for deleting an API key.
type DeleteAPIKeyInput struct {
	Key string `path:"key" doc:"API key string or prefix"`
}

// DeleteAPIKeyOutput is the output for deleting an API key (HTTP 204).
type DeleteAPIKeyOutput struct{}

// --- Set API Key Disabled ---

// SetAPIKeyDisabledInput is the input for enabling/disabling an API key.
type SetAPIKeyDisabledInput struct {
	Key  string `path:"key" doc:"API key string or name"`
	Body struct {
		Disabled bool `json:"disabled" doc:"Whether to disable the key"`
	}
}

// SetAPIKeyDisabledOutput is the output for enabling/disabling an API key.
type SetAPIKeyDisabledOutput struct {
	Body APIKeyResponse
}

// APIKeyHandler implements API key management HTTP handlers.
type APIKeyHandler struct {
	Manager *apikey.Manager
}

// CreateAPIKey creates a new API key.
func (h *APIKeyHandler) CreateAPIKey(_ context.Context, input *CreateAPIKeyInput) (*CreateAPIKeyOutput, error) {
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("API key name is required")
	}

	var expiresInDuration time.Duration
	if input.Body.ExpiresIn != "" {
		var err error
		expiresInDuration, err = time.ParseDuration(input.Body.ExpiresIn)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Invalid expires_in duration: %s", err))
		}
	}

	newKey, err := h.Manager.CreateAPIKey(input.Body.Name, expiresInDuration)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to create API key: %s", err))
	}

	return &CreateAPIKeyOutput{
		Body: APIKeyResponse{
			ID:        newKey.Key,
			Name:      newKey.Name,
			Key:       newKey.Key, // Full key shown only on creation
			CreatedAt: newKey.CreatedAt,
			ExpiresAt: newKey.ExpiresAt,
		},
	}, nil
}

// ListAPIKeys lists all API keys (without full key strings for security).
func (h *APIKeyHandler) ListAPIKeys(_ context.Context, _ *ListAPIKeysInput) (*ListAPIKeysOutput, error) {
	keys := h.Manager.ListAPIKeys()
	responseKeys := make([]APIKeyResponse, len(keys))
	for i, k := range keys {
		responseKeys[i] = APIKeyResponse{
			ID:        k.Key,
			Name:      k.Name,
			CreatedAt: k.CreatedAt,
			ExpiresAt: k.ExpiresAt,
		}
	}
	return &ListAPIKeysOutput{Body: responseKeys}, nil
}

// DeleteAPIKey deletes an API key.
func (h *APIKeyHandler) DeleteAPIKey(_ context.Context, input *DeleteAPIKeyInput) (*DeleteAPIKeyOutput, error) {
	if err := h.Manager.DeleteAPIKey(input.Key); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, huma.Error404NotFound("API key not found")
		}
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to delete API key: %s", err))
	}
	return &DeleteAPIKeyOutput{}, nil
}

// SetAPIKeyDisabled enables or disables an API key.
func (h *APIKeyHandler) SetAPIKeyDisabled(_ context.Context, input *SetAPIKeyDisabledInput) (*SetAPIKeyDisabledOutput, error) {
	updatedKey, err := h.Manager.SetAPIKeyDisabledStatus(input.Key, input.Body.Disabled)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, huma.Error404NotFound("API key not found")
		}
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to update API key: %s", err))
	}

	return &SetAPIKeyDisabledOutput{
		Body: APIKeyResponse{
			ID:        updatedKey.Key,
			Name:      updatedKey.Name,
			CreatedAt: updatedKey.CreatedAt,
			ExpiresAt: updatedKey.ExpiresAt,
		},
	}, nil
}

// Ensure APIKeyHandler implements the interface at compile time.
var _ APIKeyHandlers = (*APIKeyHandler)(nil)

// APIKeyHandlers defines the interface for API key operations.
type APIKeyHandlers interface {
	CreateAPIKey(ctx context.Context, input *CreateAPIKeyInput) (*CreateAPIKeyOutput, error)
	ListAPIKeys(ctx context.Context, input *ListAPIKeysInput) (*ListAPIKeysOutput, error)
	DeleteAPIKey(ctx context.Context, input *DeleteAPIKeyInput) (*DeleteAPIKeyOutput, error)
	SetAPIKeyDisabled(ctx context.Context, input *SetAPIKeyDisabledInput) (*SetAPIKeyDisabledOutput, error)
}
