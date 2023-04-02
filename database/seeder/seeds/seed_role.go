package seeds

import (
	"context"
	"gateway/internal/role/model"
	"gateway/internal/role/repository"

	"gorm.io/gorm"
)

func Seed_Role(db *gorm.DB) error {

	roleRepo := repository.NewRoleRepo(db)

	role := model.Role{
		Name: "admin",
	}
	roleRepo.Create(context.TODO(), role)

	role = model.Role{
		Name: "client",
	}
	roleRepo.Create(context.TODO(), role)

	return nil
}
