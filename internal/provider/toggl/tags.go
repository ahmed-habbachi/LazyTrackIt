package toggl

import (
	"context"
	"fmt"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// tagDto mirrors one entry from GET /api/v9/workspaces/{id}/tags.
type tagDto struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ListTags returns every tag defined in the user's default workspace.
// Toggl, unlike TrackIt, has no concept of project-scoped vs. system-wide
// tags: tags belong to a workspace and can be attached to any time entry in
// it, so projectID is accepted (to satisfy provider.Provider) but unused.
func (c *Client) ListTags(ctx context.Context, projectID string) ([]provider.Tag, error) {
	wsID, err := c.currentWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v9/workspaces/%d/tags", wsID)
	var dtos []tagDto
	if err := c.doJSON(ctx, "GET", path, nil, &dtos); err != nil {
		return nil, err
	}
	out := make([]provider.Tag, 0, len(dtos))
	for _, t := range dtos {
		out = append(out, provider.Tag{
			ID:   idToString(t.ID),
			Name: t.Name,
		})
	}
	return out, nil
}
