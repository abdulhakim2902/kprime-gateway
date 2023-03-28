package repository

import (
	"context"
	"fmt"
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
	fmt.Print(data)
	_ = repo.admin.Create(&data)

	return model.Client{}, nil
}

func (repo *adminRepo) GetAllClient(ctx context.Context, query map[string]interface{}) (clients []model.Client, err error) {
	_ = repo.admin.Find(&clients)
	fmt.Print(clients)
	return clients, err

}
