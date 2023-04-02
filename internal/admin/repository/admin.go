package repository

import (
	"context"
	"fmt"
	"gateway/internal/admin/model"
	_adminModel "gateway/internal/admin/model"
	_userModel "gateway/internal/user/model"

	"gorm.io/gorm"
)

type adminRepo struct {
	db *gorm.DB
}

func NewAdminRepo(db *gorm.DB) IAdminRepo {
	return &adminRepo{db}
}

func (repo *adminRepo) Register(ctx context.Context, data model.Admin) (admin model.Admin, err error) {
	_ = repo.db.Create(&data)
	return admin, nil
}

func (repo *adminRepo) CreateNewClient(ctx context.Context, data _userModel.Client) (_userModel.Client, error) {
	_ = repo.db.Create(&data)

	return _userModel.Client{}, nil
}

func (repo *adminRepo) GetAllClient(ctx context.Context, query map[string]interface{}) (clients []_userModel.Client, err error) {
	_ = repo.db.Joins("Role").Find(&clients)
	fmt.Print(clients)
	return clients, err
}

func (repo *adminRepo) GetAllRole(ctx context.Context, query map[string]interface{}) (roles []_adminModel.Role, err error) {
	_ = repo.db.Find(&roles)
	fmt.Print(roles)
	return roles, err
}
