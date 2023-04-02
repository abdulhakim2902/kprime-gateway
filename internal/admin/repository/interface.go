package repository

import (
	"context"
	"gateway/internal/admin/model"
	_adminModel "gateway/internal/admin/model"
	_userModel "gateway/internal/user/model"
)

type IAdminRepo interface {
	Register(ctx context.Context, data model.Admin) (admin model.Admin, err error)
	CreateNewClient(ctx context.Context, data _userModel.Client) (client _userModel.Client, err error)
	GetAllClient(context.Context, map[string]interface{}) ([]_userModel.Client, error)
	GetAllRole(context.Context, map[string]interface{}) ([]_adminModel.Role, error)
}
