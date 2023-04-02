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

func (repo *roleRepo) Create(ctx context.Context, data model.Role) (admin model.Role, err error) {
	_ = repo.db.Create(&data)
	return admin, nil
}
