package repository

import (
	"context"
	"fmt"
	"gateway/internal/admin/model"

	"gorm.io/gorm"
)

type authRepo struct {
	db *gorm.DB
}

func NewAuthRepo(db *gorm.DB) IAuthRepo {
	return &authRepo{db}
}

func (a authRepo) GetOneUserByEmail(ctx context.Context, email string) (user model.Client, err error) {
	result := a.db.Where(&model.Client{Email: email}).First(&user)
	if result.Error != nil {
		return user, result.Error
	}
	if user == (model.Client{}) {
		return user, fmt.Errorf("user with the email %s is not found", email)
	}
	fmt.Print(user.ID, user.Email)
	return user, nil
}

func (a authRepo) GetUser(ctx context.Context, query map[string]interface{}) (users []model.Client, err error) {
	return users, nil
}