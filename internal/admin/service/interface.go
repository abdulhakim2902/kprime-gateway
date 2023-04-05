package service

import (
	"context"
	"gateway/internal/admin/model"
	_adminModel "gateway/internal/admin/model"
	_roleModel "gateway/internal/role/model"
	_userModel "gateway/internal/user/model"
	"net/url"
)

type IAdminService interface {
	Register(context.Context, model.RegisterAdmin) (model.Admin, error)
	CreateNewClient(context.Context, _userModel.CreateClient) (_userModel.APIKeys, error)
	CreateNewRole(context.Context, _roleModel.CreateRole) (_roleModel.ResponseRole, error)
	DeleteRole(context.Context, int) (_roleModel.ResponseRole, error)
	DeleteClient(context.Context, int) (_userModel.ResponseClient, error)
	GetAllClient(context.Context, url.Values) ([]_userModel.Client, error)
	GetAllRole(context.Context, url.Values) ([]_adminModel.Role, error)
}
