package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	kerrors "github.com/jmylchreest/keylightd/internal/errors"
	"github.com/jmylchreest/keylightd/internal/group"
	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// --- List Groups ---

// ListGroupsInput is the input for listing all groups.
type ListGroupsInput struct{}

// ListGroupsOutput is the output for listing all groups.
// Returns groups as an array for backward compatibility with the GNOME extension.
type ListGroupsOutput struct {
	Body []GroupResponse
}

// --- Create Group ---

// CreateGroupInput is the input for creating a new group.
type CreateGroupInput struct {
	Body struct {
		Name     string   `json:"name" doc:"Display name for the group" minLength:"1"`
		LightIDs []string `json:"light_ids,omitempty" doc:"Optional list of light IDs to include"`
	}
}

// CreateGroupOutput is the output for creating a new group (HTTP 201).
// The 201 status is set via DefaultStatus in the operation registration.
type CreateGroupOutput struct {
	Body GroupResponse
}

// --- Get Group ---

// GetGroupInput is the input for getting a single group.
type GetGroupInput struct {
	ID string `path:"id" doc:"Group identifier (UUID or name)"`
}

// GetGroupOutput is the output for getting a single group.
type GetGroupOutput struct {
	Body GroupResponse
}

// --- Delete Group ---

// DeleteGroupInput is the input for deleting a group.
type DeleteGroupInput struct {
	ID string `path:"id" doc:"Group identifier"`
}

// DeleteGroupOutput is the output for deleting a group (HTTP 204).
type DeleteGroupOutput struct{}

// --- Set Group Lights ---

// SetGroupLightsInput is the input for setting which lights belong to a group.
type SetGroupLightsInput struct {
	ID   string `path:"id" doc:"Group identifier"`
	Body struct {
		LightIDs []string `json:"light_ids" doc:"List of light IDs to assign to the group"`
	}
}

// SetGroupLightsOutput is the output for setting group lights.
type SetGroupLightsOutput struct {
	Body StatusResponse
}

// --- Set Group State ---

// SetGroupStateInput is the input for setting a group's state.
// The ID path parameter supports comma-separated IDs/names for multi-group targeting.
type SetGroupStateInput struct {
	ID   string `path:"id" doc:"Group identifier(s), comma-separated for multi-target"`
	Body struct {
		On          *bool `json:"on,omitempty" doc:"Power state for all lights in the group"`
		Brightness  *int  `json:"brightness,omitempty" doc:"Brightness level (0-100) for all lights"`
		Temperature *int  `json:"temperature,omitempty" doc:"Color temperature for all lights"`
	}
}

// SetGroupStateOutput is the output for setting group state.
// On success returns 200 with {"status": "ok"}.
// On partial failure returns 207 with {"status": "partial", "errors": [...]}.
// This uses a raw writer because Huma doesn't natively support 207 Multi-Status.
type SetGroupStateOutput struct {
	Body any // Either StatusResponse or PartialStatusResponse
}

// GroupHandler implements group-related HTTP handlers.
type GroupHandler struct {
	Groups *group.Manager
	Lights keylight.LightManager
}

// ListGroups returns all groups as an array.
func (h *GroupHandler) ListGroups(_ context.Context, _ *ListGroupsInput) (*ListGroupsOutput, error) {
	groups := h.Groups.GetGroups()
	return &ListGroupsOutput{
		Body: GroupsFromInternal(groups),
	}, nil
}

// CreateGroup creates a new group and returns it with HTTP 201.
func (h *GroupHandler) CreateGroup(ctx context.Context, input *CreateGroupInput) (*CreateGroupOutput, error) {
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("Group name is required")
	}

	grp, err := h.Groups.CreateGroup(ctx, input.Body.Name, input.Body.LightIDs)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to create group: %s", err))
	}

	return &CreateGroupOutput{
		Body: GroupFromInternal(grp),
	}, nil
}

// GetGroup returns a single group by ID.
func (h *GroupHandler) GetGroup(_ context.Context, input *GetGroupInput) (*GetGroupOutput, error) {
	grp, err := h.Groups.GetGroup(input.ID)
	if err != nil {
		return nil, huma.Error404NotFound(fmt.Sprintf("Group not found: %s", err))
	}
	return &GetGroupOutput{Body: GroupFromInternal(grp)}, nil
}

// DeleteGroup deletes a group and returns HTTP 204.
func (h *GroupHandler) DeleteGroup(_ context.Context, input *DeleteGroupInput) (*DeleteGroupOutput, error) {
	if err := h.Groups.DeleteGroup(input.ID); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, huma.Error404NotFound("Group not found")
		}
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to delete group: %s", err))
	}
	return &DeleteGroupOutput{}, nil
}

// SetGroupLights sets which lights belong to a group.
func (h *GroupHandler) SetGroupLights(ctx context.Context, input *SetGroupLightsInput) (*SetGroupLightsOutput, error) {
	if err := h.Groups.SetGroupLights(ctx, input.ID, input.Body.LightIDs); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, huma.Error404NotFound("Group or light not found")
		}
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to set group lights: %s", err))
	}
	return &SetGroupLightsOutput{
		Body: StatusResponse{Status: "ok"},
	}, nil
}

// SetGroupState sets the state for one or more groups (comma-separated IDs/names).
// Returns 200 on full success, 207 on partial failure.
// This is implemented as a raw handler because Huma doesn't support 207.
func (h *GroupHandler) SetGroupState(ctx context.Context, input *SetGroupStateInput) (*SetGroupStateOutput, error) {
	// Parse comma-separated group keys
	groupKeys := strings.Split(input.ID, ",")
	var matchedGroups []*group.Group
	var notFound []string
	groupSeen := make(map[string]bool)

	for _, key := range groupKeys {
		key = strings.TrimSpace(key)
		// Try by ID
		grp, err := h.Groups.GetGroup(key)
		if err == nil {
			if !groupSeen[grp.ID] {
				matchedGroups = append(matchedGroups, grp)
				groupSeen[grp.ID] = true
			}
			continue
		}
		// Try by name
		byName := h.Groups.GetGroupsByName(key)
		if len(byName) > 0 {
			for _, g := range byName {
				if !groupSeen[g.ID] {
					matchedGroups = append(matchedGroups, g)
					groupSeen[g.ID] = true
				}
			}
		} else {
			notFound = append(notFound, key)
		}
	}

	if len(matchedGroups) == 0 {
		return nil, huma.Error404NotFound(fmt.Sprintf("No groups found for: %v", notFound))
	}

	var errs []string
	for _, grp := range matchedGroups {
		if input.Body.On != nil {
			if err := h.Groups.SetGroupState(ctx, grp.ID, *input.Body.On); err != nil {
				errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
			}
		}
		if input.Body.Brightness != nil {
			if err := h.Groups.SetGroupBrightness(ctx, grp.ID, *input.Body.Brightness); err != nil {
				errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
			}
		}
		if input.Body.Temperature != nil {
			if err := h.Groups.SetGroupTemperature(ctx, grp.ID, *input.Body.Temperature); err != nil {
				errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
			}
		}
	}

	if len(errs) > 0 {
		return &SetGroupStateOutput{
			Body: PartialStatusResponse{Status: "partial", Errors: errs},
		}, nil
	}

	return &SetGroupStateOutput{
		Body: StatusResponse{Status: "ok"},
	}, nil
}

// SetGroupStateRaw is the raw HTTP handler for SetGroupState.
// This is needed because Huma doesn't natively support 207 Multi-Status.
// It wraps the typed handler and writes the appropriate status code.
func (h *GroupHandler) SetGroupStateRaw(api huma.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		// Parse path parameter - Chi uses {id}
		groupParam := r.PathValue("id")
		if groupParam == "" {
			// Try Chi's URL param
			groupParam = chi_URLParam(r, "id")
		}

		groupKeys := strings.Split(groupParam, ",")
		var matchedGroups []*group.Group
		var notFound []string
		groupSeen := make(map[string]bool)

		for _, key := range groupKeys {
			key = strings.TrimSpace(key)
			grp, err := h.Groups.GetGroup(key)
			if err == nil {
				if !groupSeen[grp.ID] {
					matchedGroups = append(matchedGroups, grp)
					groupSeen[grp.ID] = true
				}
				continue
			}
			byName := h.Groups.GetGroupsByName(key)
			if len(byName) > 0 {
				for _, g := range byName {
					if !groupSeen[g.ID] {
						matchedGroups = append(matchedGroups, g)
						groupSeen[g.ID] = true
					}
				}
			} else {
				notFound = append(notFound, key)
			}
		}

		if len(matchedGroups) == 0 {
			http.Error(w, fmt.Sprintf("No groups found for: %v", notFound), http.StatusNotFound)
			return
		}

		var reqBody struct {
			On          *bool `json:"on,omitempty"`
			Brightness  *int  `json:"brightness,omitempty"`
			Temperature *int  `json:"temperature,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var errs []string
		for _, grp := range matchedGroups {
			if reqBody.On != nil {
				if err := h.Groups.SetGroupState(r.Context(), grp.ID, *reqBody.On); err != nil {
					errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
				}
			}
			if reqBody.Brightness != nil {
				if err := h.Groups.SetGroupBrightness(r.Context(), grp.ID, *reqBody.Brightness); err != nil {
					errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
				}
			}
			if reqBody.Temperature != nil {
				if err := h.Groups.SetGroupTemperature(r.Context(), grp.ID, *reqBody.Temperature); err != nil {
					errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if len(errs) > 0 {
			w.WriteHeader(http.StatusMultiStatus) // 207
			json.NewEncoder(w).Encode(PartialStatusResponse{Status: "partial", Errors: errs})
			return
		}
		json.NewEncoder(w).Encode(StatusResponse{Status: "ok"})
	}
}

// chi_URLParam extracts a URL parameter from a Chi request.
// This is a helper to avoid importing chi directly in handlers.
func chi_URLParam(r *http.Request, key string) string {
	// Chi stores URL params in the request context via chi.URLParam
	// but we can use PathValue which Chi also supports in Go 1.22+
	return r.PathValue(key)
}

// Ensure GroupHandler implements the interface at compile time.
var _ GroupHandlers = (*GroupHandler)(nil)

// GroupHandlers defines the interface for group operations.
type GroupHandlers interface {
	ListGroups(ctx context.Context, input *ListGroupsInput) (*ListGroupsOutput, error)
	CreateGroup(ctx context.Context, input *CreateGroupInput) (*CreateGroupOutput, error)
	GetGroup(ctx context.Context, input *GetGroupInput) (*GetGroupOutput, error)
	DeleteGroup(ctx context.Context, input *DeleteGroupInput) (*DeleteGroupOutput, error)
	SetGroupLights(ctx context.Context, input *SetGroupLightsInput) (*SetGroupLightsOutput, error)
	SetGroupState(ctx context.Context, input *SetGroupStateInput) (*SetGroupStateOutput, error)
	SetGroupStateRaw(api huma.API) http.HandlerFunc
}
