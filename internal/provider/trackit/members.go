package trackit

import (
	"context"
	"net/url"
	"strconv"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// userSettingsMeDto is the response of GET /api/UserSettings/me, which
// resolves identity server-side from the bearer token. Unlike
// /api/Members/ByUsername/{username} (which turned out to ignore its path
// parameter and always return the same member, regardless of caller), this
// endpoint has been confirmed to return the actual caller's own identity.
type userSettingsMeDto struct {
	UserID   string `json:"userId"`
	Email    string `json:"email"`
	UserName string `json:"userName"`
}

// profileDto is the response of GET /api/Profile/{userId}.
type profileDto struct {
	MemberID int32  `json:"memberId"`
	FullName string `json:"fullname"`
	Email    string `json:"email"`
}

// Me resolves the authenticated user's TrackIt member record.
func (c *Client) Me(ctx context.Context) (provider.Member, error) {
	var settings userSettingsMeDto
	if err := c.doJSON(ctx, "GET", "/api/UserSettings/me", nil, &settings); err != nil {
		return provider.Member{}, err
	}

	var profile profileDto
	path := "/api/Profile/" + url.PathEscape(settings.UserID)
	if err := c.doJSON(ctx, "GET", path, nil, &profile); err != nil {
		return provider.Member{}, err
	}

	m := provider.Member{
		ID:       strconv.Itoa(int(profile.MemberID)),
		UserID:   settings.UserID,
		FullName: profile.FullName,
		Username: settings.UserName,
		Email:    settings.Email,
	}
	c.mu.Lock()
	c.me = &m
	c.mu.Unlock()
	return m, nil
}
