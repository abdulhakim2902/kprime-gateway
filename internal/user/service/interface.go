package service

import (
	"context"
	"gateway/internal/user/types"
	"gateway/pkg/hmac"
)

type IAuthService interface {
	Login(context.Context, types.AuthRequest) (*types.AuthResponse, *types.User, error)
	RefreshToken(context.Context, types.JwtClaim) (*types.AuthResponse, *types.User, error)
	LoginWithSignature(ctx context.Context, sig hmac.Signature) (res *types.AuthResponse, user *types.User, err error)
}

type IUserService interface {
	SyncMemDB(context.Context, interface{}) error
}
