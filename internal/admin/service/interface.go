package service

import (
	"context"
	"gateway/internal/user/model"
	"net/url"
)

type IAdminService interface {
	CreateNewClient(context.Context, model.CreateClient) (model.Client, error)
	GetAllClient(context.Context, url.Values) ([]model.Client, error)
}
