package trackit

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/ahmed-habbachi/lazytrackit/internal/auth"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

type memberDto struct {
	ID       int32  `json:"id"`
	UserID   string `json:"userId"`
	FullName string `json:"fullName"`
	UserName string `json:"userName"`
}

type memberDetailsVm struct {
	Member *memberDto `json:"member"`
}

// Me resolves the authenticated user's TrackIt member record by decoding
// preferred_username out of the access token and looking it up.
func (c *Client) Me(ctx context.Context) (provider.Member, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return provider.Member{}, err
	}
	claims, err := auth.ParseJWTClaims(token)
	if err != nil {
		return provider.Member{}, err
	}
	username := auth.StringClaim(claims, "preferred_username")
	if username == "" {
		return provider.Member{}, fmt.Errorf("access token has no preferred_username claim")
	}
	email := auth.StringClaim(claims, "email")

	var vm memberDetailsVm
	path := "/api/Members/ByUsername/" + url.PathEscape(username)
	if err := c.doJSON(ctx, "GET", path, nil, &vm); err != nil {
		return provider.Member{}, err
	}
	if vm.Member == nil {
		return provider.Member{}, fmt.Errorf("no member found for username %q", username)
	}
	m := provider.Member{
		ID:       strconv.Itoa(int(vm.Member.ID)),
		UserID:   vm.Member.UserID,
		FullName: vm.Member.FullName,
		Username: vm.Member.UserName,
		Email:    email,
	}
	c.mu.Lock()
	c.me = &m
	c.mu.Unlock()
	return m, nil
}
