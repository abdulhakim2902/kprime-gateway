package service

import (
	"context"
	"gateway/internal/admin/model"
	_adminModel "gateway/internal/admin/model"
	_userModel "gateway/internal/user/model"
	"net/url"
)

type IAdminService interface {
	Register(context.Context, model.RegisterAdmin) (model.Admin, error)
	CreateNewClient(context.Context, _userModel.CreateClient) (_userModel.APIKeys, error)
	GetAllClient(context.Context, url.Values) ([]_userModel.Client, error)
	GetAllRole(context.Context, url.Values) ([]_adminModel.Role, error)
}
