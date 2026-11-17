package trackit

import (
	"context"
	"strings"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// BackendVersion returns the TrackIt server's release version, from
// GET /api/AppConfig/version (a JSON string).
func (c *Client) BackendVersion(ctx context.Context) (string, error) {
	var v string
	if err := c.doJSON(ctx, "GET", "/api/AppConfig/version", nil, &v); err != nil {
		return "", err
	}
	return strings.TrimSpace(v), nil
}

var _ provider.BackendVersioner = (*Client)(nil)
