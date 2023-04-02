package seeds

import (
	"context"
	"gateway/internal/role/model"
	"gateway/internal/role/repository"

	"gorm.io/gorm"
)

func Seed_Role(db *gorm.DB) error {

	role := model.Role{
		Name: "admin",
	}

	roleRepo := repository.NewRoleRepo(db)

	roleRepo.Create(context.TODO(), role)

	return nil
}
