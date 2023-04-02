package seeds

import (
	"context"
	"gateway/internal/admin/model"
	"gateway/internal/admin/repository"

	"gorm.io/gorm"

	"golang.org/x/crypto/bcrypt"
)

func Seed_Admin(db *gorm.DB) error {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	admin := model.Admin{
		Name:     "admin",
		Email:    "admin@mail.com",
		Password: string(hashedPassword),
		RoleId:   1,
	}

	adminRepo := repository.NewAdminRepo(db)

	adminRepo.Register(context.TODO(), admin)

	return nil
}
