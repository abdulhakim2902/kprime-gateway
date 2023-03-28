package repository

import (
	"context"
	"gateway/internal/user/model"
)

type IAdminRepo interface {
	CreateNewClient(ctx context.Context, data model.Client) (client model.Client, err error)
	GetAllClient(context.Context, map[string]interface{}) ([]model.Client, error)
}
