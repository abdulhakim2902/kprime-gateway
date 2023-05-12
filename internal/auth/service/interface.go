package service

import (
	"context"
	"gateway/internal/auth/types"
)

type IAuthService interface {
	Login(context.Context, types.AuthRequest) (*types.AuthResponse, error)
	RefreshToken(context.Context, types.JwtClaim) (*types.AuthResponse, error)
	ClaimJWT(string) (types.JwtClaim, error)
}
