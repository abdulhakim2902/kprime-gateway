package repository

import (
	"context"
	"gateway/internal/role/model"

	"gorm.io/gorm"
)

type roleRepo struct {
	db *gorm.DB
}

func NewRoleRepo(db *gorm.DB) IRoleRepo {
	return &roleRepo{db}
}

func (repo *roleRepo) Create(ctx context.Context, data model.Role) (role model.Role, err error) {
	_ = repo.db.Create(&data)
	return role, nil
}

func (repo *roleRepo) GetByName(ctx context.Context, name string) (role model.Role, err error) {
	result := repo.db.Where("name = ?", name).First(&role)

	return role, result.Error
}
