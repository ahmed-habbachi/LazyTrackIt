package weekbooking

import (
	"context"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// projectDto mirrors one entry from GET /api/v1/projects.
type projectDto struct {
	ID            int64  `json:"id"`
	ProjectNumber string `json:"project_number"`
	Name          string `json:"name"`
	Active        bool   `json:"active"`
}

// ListProjects returns the caller's active projects. The API has no concept
// of a client/customer per project (just a project_number and name), and no
// per-project color, so those fields are left blank.
func (c *Client) ListProjects(ctx context.Context) ([]provider.Project, error) {
	var dtos []projectDto
	if err := c.doJSON(ctx, "GET", "/api/v1/projects", nil, &dtos); err != nil {
		return nil, err
	}
	out := make([]provider.Project, 0, len(dtos))
	for _, p := range dtos {
		out = append(out, provider.Project{
			ID:     idToString(p.ID),
			Name:   formatProjectName(p),
			Active: p.Active,
		})
	}
	return out, nil
}

// formatProjectName prefixes the project number, e.g. "8131 — Rel 8.6
// CP4000/5x00/CM6000", since the number is how the customer's team actually
// identifies a project day to day.
func formatProjectName(p projectDto) string {
	if p.ProjectNumber == "" {
		return p.Name
	}
	return p.ProjectNumber + " — " + p.Name
}
