package service

import (
	"context"
	"gateway/internal/user/types"
	"gateway/pkg/ws"
)

type IAuthService interface {
	Login(context.Context, types.AuthRequest, *ws.Client) (*types.AuthResponse, error)
	RefreshToken(context.Context, types.JwtClaim, *ws.Client) (*types.AuthResponse, error)
	ClaimJWT(string, *ws.Client) (types.JwtClaim, error)
}
