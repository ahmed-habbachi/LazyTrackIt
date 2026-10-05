package toggl

import (
	"context"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// meDto mirrors the relevant fields of GET /api/v9/me.
type meDto struct {
	ID                 int64  `json:"id"`
	Email              string `json:"email"`
	Fullname           string `json:"fullname"`
	DefaultWorkspaceID int64  `json:"default_workspace_id"`
}

// Me resolves the authenticated user and caches their default workspace,
// which every other endpoint needs but the Provider interface has no slot
// for.
func (c *Client) Me(ctx context.Context) (provider.Member, error) {
	var dto meDto
	if err := c.doJSON(ctx, "GET", "/api/v9/me", nil, &dto); err != nil {
		return provider.Member{}, err
	}
	m := provider.Member{
		ID:       idToString(dto.ID),
		UserID:   idToString(dto.ID),
		FullName: dto.Fullname,
		Email:    dto.Email,
	}
	c.mu.Lock()
	c.me = &m
	c.workspaceID = dto.DefaultWorkspaceID
	c.mu.Unlock()
	return m, nil
}
