package service

import (
	"context"
	"gateway/internal/auth/model"
	"gateway/internal/auth/repository"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	repo repository.IAuthRepo
}

func NewAuthService(repo repository.IAuthRepo) IAuthService {
	return &authService{repo}
}

func (s authService) Login(ctx context.Context, data model.LoginRequest) (signedToken string, err error) {
	user, err := s.repo.GetOneUserByEmail(ctx, data.Email)
	if (err != nil) {
		return "", err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(data.Password))
	if err != nil {
		return "", bcrypt.ErrMismatchedHashAndPassword
	}
	token := jwt.New(jwt.SigningMethodHS256)
	signedToken, err = token.SignedString([]byte(user.ClientId))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}