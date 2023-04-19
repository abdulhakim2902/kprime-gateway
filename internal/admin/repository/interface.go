package repository

import (
	"context"
	"gateway/internal/admin/model"
	_adminModel "gateway/internal/admin/model"
	_userModel "gateway/internal/user/model"
)

type IAdminRepo interface {
	Register(ctx context.Context, data model.Admin) (admin model.Admin, err error)
	CreateNewClient(ctx context.Context, data _userModel.Client) (client _userModel.APIKeys, err error)
	CreateNewRole(ctx context.Context, data _adminModel.Role) (role _adminModel.ResponseRole, err error)
	CreateNewPermission(ctx context.Context, data _adminModel.Permission) (role _adminModel.ResponsePermission, err error)
	CreateNewCasbin(ctx context.Context, data _adminModel.Casbin, id int) (client _adminModel.ResponseCasbin, err error)
	DetailRole(ctx context.Context, id int) ([]_adminModel.Role, error)
	UpdateRole(ctx context.Context, data _adminModel.Role, id int) (role _adminModel.ResponseRole, err error)
	DeleteRole(ctx context.Context, id int) (role _adminModel.ResponseRole, err error)
	DeleteClient(ctx context.Context, id int) (client _userModel.ResponseClient, err error)
	DeleteCasbin(ctx context.Context, id int) (casbin _adminModel.ResponseCasbin, err error)
	GetAllClient(context.Context, map[string]interface{}) ([]_userModel.Client, error)
	GetAllRole(context.Context, map[string]interface{}) ([]_adminModel.Role, error)
	GetAllPermission(context.Context, map[string]interface{}) ([]_adminModel.Permission, error)
	GetAllCashbin(context.Context, map[string]interface{}) ([]_adminModel.Casbin, error)
	GetByName(context.Context, string) (model.Admin, error)
	GetById(context.Context, int) (client _userModel.Client, err error)
	UpdateClient(ctx context.Context, data _userModel.Client, id int) (client _userModel.ResponseClient, err error)
	UpdatePermission(ctx context.Context, data _adminModel.Permission, id int) (client _adminModel.ResponsePermission, err error)
}
