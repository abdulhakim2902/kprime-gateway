package repository

import (
	"context"
	"gateway/internal/admin/model"
	_adminModel "gateway/internal/admin/model"
	_roleModel "gateway/internal/admin/model"
	_userModel "gateway/internal/user/model"
)

type IAdminRepo interface {
	Register(ctx context.Context, data model.Admin) (admin model.Admin, err error)
	CreateNewClient(ctx context.Context, data _userModel.Client) (client _userModel.APIKeys, err error)
	CreateNewRole(ctx context.Context, data _roleModel.Role) (role _roleModel.ResponseRole, err error)
	DetailRole(ctx context.Context, id int) ([]_adminModel.Role, error)
	DeleteRole(ctx context.Context, id int) (role _roleModel.ResponseRole, err error)
	DeleteClient(ctx context.Context, id int) (client _userModel.ResponseClient, err error)
	GetAllClient(context.Context, map[string]interface{}) ([]_userModel.Client, error)
	GetAllRole(context.Context, map[string]interface{}) ([]_adminModel.Role, error)
	GetByName(context.Context, string) (model.Admin, error)
}
