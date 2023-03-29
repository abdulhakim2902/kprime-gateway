package repository

import (
	"context"
	"fmt"
	_adminModel "gateway/internal/admin/model"
	_userModel "gateway/internal/user/model"

	"gorm.io/gorm"
)

type authRepo struct {
	db *gorm.DB
}

func NewAuthRepo(db *gorm.DB) IAuthRepo {
	return &authRepo{db}
}

func (a authRepo) GetOneUserByEmail(ctx context.Context, email string) (user _userModel.Client, err error) {
	result := a.db.Joins("Role").Where(&_userModel.Client{Email: email}).First(&user)
	if result.Error != nil {
		return user, result.Error
	}
	if user == (_userModel.Client{}) {
		return user, fmt.Errorf("user with the email %s is not found", email)
	}
	return user, nil
}

func (a authRepo) GetAdminByEmail(ctx context.Context, email string) (admin _adminModel.Admin, err error) {
	result := a.db.Joins("Role").Where(&_adminModel.Admin{Email: email}).First(&admin)
	if result.Error != nil {
		return admin, result.Error
	}
	if admin == (_adminModel.Admin{}) {
		return admin, fmt.Errorf("user with the email %s is not found", email)
	}
	return admin, nil
}


func (a authRepo) GetUser(ctx context.Context, query map[string]interface{}) (users []_userModel.Client, err error) {
	return users, nil
}