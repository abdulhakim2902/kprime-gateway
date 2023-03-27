package repository

import (
	"context"
	"gateway/internal/admin/model"
)

type Repo interface {
	CreateNewClient(ctx context.Context, data model.Client) (client model.Client, err error)
}
