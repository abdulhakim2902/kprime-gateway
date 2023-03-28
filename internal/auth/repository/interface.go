package repository

import (
	"context"
	"gateway/internal/admin/model"
)

type IAuthRepo interface {
	GetUser(context.Context, map[string]interface{}) ([]model.Client, error)
	GetOneUserByEmail(context.Context, string) (model.Client, error)
}