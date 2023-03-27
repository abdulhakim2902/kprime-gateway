package repository

import (
	"context"
	"gateway/internal/admin/model"

	"gorm.io/gorm"
)

type adminRepo struct {
	admin *gorm.DB
}

func NewAdminRepo(db *gorm.DB) Repo {
	return &adminRepo{db}
}

func (repo *adminRepo) CreateNewClient(ctx context.Context, data model.Client) (model.Client, error) {
	client := &model.Client{
		Name: data.Name,
	}
	_ = repo.admin.Create(client)

	return model.Client{}, nil
}
