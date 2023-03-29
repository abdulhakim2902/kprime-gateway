package repository

import (
	"context"
	"fmt"
	_adminModel "gateway/internal/admin/model"
	_authModel "gateway/internal/auth/model"
	_userModel "gateway/internal/user/model"

	"github.com/google/uuid"

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

func (a authRepo) GenerateAuthDetail(ctx context.Context, userId uint) (auth _authModel.TokenAuth, err error) {
	auth.UserID = userId
	auth.AuthUUID = uuid.NewString()
	results := a.db.Create(&auth)
	if results.Error != nil {
		return _authModel.TokenAuth{}, results.Error
	}
	return auth, nil
}

func (a authRepo) InvalidateToken(ctx context.Context, userID uint, authID string) (error) {
	a.db.Where(&_authModel.TokenAuth{
		AuthUUID: authID,
		UserID: userID,
	}).Delete(&_authModel.TokenAuth{})
	return nil
}