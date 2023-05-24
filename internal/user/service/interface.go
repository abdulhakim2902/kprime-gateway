package service

import (
	"context"
	"gateway/internal/user/types"
)

type IAuthService interface {
	Login(context.Context, types.AuthRequest) (*types.AuthResponse, *types.User, error)
	RefreshToken(context.Context, types.JwtClaim) (*types.AuthResponse, *types.User, error)
}
