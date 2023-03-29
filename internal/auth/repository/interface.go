package repository

import (
	"context"
	_adminModel "gateway/internal/admin/model"
	_userModel "gateway/internal/user/model"
)

type IAuthRepo interface {
	GetUser(context.Context, map[string]interface{}) ([]_userModel.Client, error)
	GetOneUserByEmail(context.Context, string) (_userModel.Client, error)
	GetAdminByEmail(context.Context, string) (_adminModel.Admin, error)
}