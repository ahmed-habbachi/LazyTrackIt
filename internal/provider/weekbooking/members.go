package weekbooking

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// weekStatusDto mirrors the response of GET /api/v1/weeks/status.
type weekStatusDto struct {
	UserID string `json:"user_id"`
}

// Me resolves the authenticated user. The week-booking spec has no
// dedicated "whoami" endpoint, so this piggybacks on weeks/status for the
// current ISO week with user_id omitted (which the backend resolves to the
// caller): the response's user_id is the only identity the API exposes
// outside of a full session/OIDC login. FullName/Email/Username aren't
// available this way, so Member.DisplayName() falls back to showing
// nothing rather than a guess.
func (c *Client) Me(ctx context.Context) (provider.Member, error) {
	year, week := time.Now().ISOWeek()
	q := url.Values{
		"year": {fmt.Sprintf("%d", year)},
		"week": {fmt.Sprintf("%d", week)},
	}
	path := "/api/v1/weeks/status?" + q.Encode()

	var dto weekStatusDto
	if err := c.doJSON(ctx, "GET", path, nil, &dto); err != nil {
		return provider.Member{}, err
	}
	m := provider.Member{
		ID:     dto.UserID,
		UserID: dto.UserID,
	}
	c.mu.Lock()
	c.me = &m
	c.mu.Unlock()
	return m, nil
}
