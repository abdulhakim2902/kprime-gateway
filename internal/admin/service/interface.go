package service

import (
	"context"
	"gateway/internal/admin/model"
)

type AdminService interface {
	CreateNewClient(context.Context, model.CreateClient) model.Client
}
