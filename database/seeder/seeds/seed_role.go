package seeds

import (
	"context"
	"gateway/internal/role/model"
	"gateway/internal/role/repository"

	"gorm.io/gorm"
)

func Seed_Role(db *gorm.DB) error {

	roleRepo := repository.NewRoleRepo(db)

	// First or Create
	_, err := roleRepo.GetByName(context.TODO(), "admin")
	if err != nil {
		role := model.Role{
			Name: "admin",
		}
		roleRepo.Create(context.TODO(), role)
	}

	_, err = roleRepo.GetByName(context.TODO(), "market_maker")
	if err != nil {
		role := model.Role{
			Name: "market_maker",
		}
		roleRepo.Create(context.TODO(), role)
	}

	_, err = roleRepo.GetByName(context.TODO(), "user")
	if err != nil {
		role := model.Role{
			Name: "user",
		}
		roleRepo.Create(context.TODO(), role)
	}

	return nil
}
