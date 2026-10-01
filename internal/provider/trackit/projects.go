package trackit

import (
	"context"
	"strconv"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

type projectDto struct {
	ID         int32       `json:"id"`
	Name       string      `json:"name"`
	ClientName string      `json:"clientName"`
	Color      looseString `json:"color"`
	IsActive   bool        `json:"isActive"`
}

type projectsListVm struct {
	Projects []projectDto `json:"projects"`
	Count    int32        `json:"count"`
}

// ListProjects returns all projects visible to the authenticated user.
func (c *Client) ListProjects(ctx context.Context) ([]provider.Project, error) {
	var vm projectsListVm
	if err := c.doJSON(ctx, "GET", "/api/Projects", nil, &vm); err != nil {
		return nil, err
	}
	out := make([]provider.Project, 0, len(vm.Projects))
	for _, p := range vm.Projects {
		out = append(out, provider.Project{
			ID:         strconv.Itoa(int(p.ID)),
			Name:       p.Name,
			ClientName: p.ClientName,
			Color:      string(p.Color),
			Active:     p.IsActive,
		})
	}
	return out, nil
}
