package toggl

import (
	"context"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// projectDto mirrors the relevant fields of one entry from GET
// /api/v9/me/projects.
type projectDto struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Active   bool   `json:"active"`
	ClientID *int64 `json:"client_id"`
}

// clientDto mirrors one entry from GET /api/v9/me/clients.
type clientDto struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ListProjects returns every project across all of the user's workspaces.
// Toggl's project objects only carry a client_id, not a client name, so this
// makes a second call to resolve names and joins them locally.
func (c *Client) ListProjects(ctx context.Context) ([]provider.Project, error) {
	var projectDtos []projectDto
	if err := c.doJSON(ctx, "GET", "/api/v9/me/projects", nil, &projectDtos); err != nil {
		return nil, err
	}

	var clientDtos []clientDto
	if err := c.doJSON(ctx, "GET", "/api/v9/me/clients", nil, &clientDtos); err != nil {
		return nil, err
	}
	clientNames := make(map[int64]string, len(clientDtos))
	for _, cl := range clientDtos {
		clientNames[cl.ID] = cl.Name
	}

	out := make([]provider.Project, 0, len(projectDtos))
	for _, p := range projectDtos {
		var clientName string
		if p.ClientID != nil {
			clientName = clientNames[*p.ClientID]
		}
		out = append(out, provider.Project{
			ID:         idToString(p.ID),
			Name:       p.Name,
			ClientName: clientName,
			Color:      p.Color,
			Active:     p.Active,
		})
	}
	return out, nil
}
