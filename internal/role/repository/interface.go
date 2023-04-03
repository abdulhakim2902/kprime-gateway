package repository

import (
	"context"
	"gateway/internal/role/model"
)

type IRoleRepo interface {
	Create(ctx context.Context, data model.Role) (role model.Role, err error)
	GetByName(ctx context.Context, name string) (role model.Role, err error)
}
