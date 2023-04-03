package seeds

import (
	"context"
	"gateway/internal/admin/model"
	"gateway/internal/admin/repository"
	roleRepository "gateway/internal/role/repository"

	"gorm.io/gorm"

	"golang.org/x/crypto/bcrypt"
)

func Seed_Admin(db *gorm.DB) error {
	adminRepo := repository.NewAdminRepo(db)

	// First Or Create
	_, err := adminRepo.GetByName(context.TODO(), "admin")
	if err != nil {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)

		roleRepo := roleRepository.NewRoleRepo(db)
		role, _ := roleRepo.GetByName(context.TODO(), "admin")

		admin := model.Admin{
			Name:     "admin",
			Email:    "admin@mail.com",
			Password: string(hashedPassword),
			RoleId:   role.ID,
		}

		adminRepo.Register(context.TODO(), admin)
	}

	return nil
}
