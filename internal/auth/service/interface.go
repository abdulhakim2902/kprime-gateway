package service

import (
	"context"
	"gateway/internal/auth/model"
)

type IAuthService interface {
	Login(context.Context, model.LoginRequest) (string, error)
	AdminLogin(context.Context, model.LoginRequest) (string, error)
	Logout(context.Context) (error)
}