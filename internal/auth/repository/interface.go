package repository

import (
	"context"
	_adminModel "gateway/internal/admin/model"
	_authModel "gateway/internal/auth/model"
	_userModel "gateway/internal/user/model"
)

type IAuthRepo interface {
	GetUser(context.Context, map[string]interface{}) ([]_userModel.Client, error)
	GetOneUserByEmail(context.Context, string) (_userModel.Client, error)
	GetAdminByEmail(context.Context, string) (_adminModel.Admin, error)
	GenerateAuthDetail(context.Context, uint) (_authModel.TokenAuth, error)
	InvalidateToken(context.Context, uint, string) (error)
}