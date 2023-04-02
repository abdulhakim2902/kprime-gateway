package service

import (
	"context"
	"gateway/internal/auth/model"
	"gateway/internal/auth/repository"
	"time"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo repository.IAuthRepo
}

func NewAuthService(repo repository.IAuthRepo) IAuthService {
	return &AuthService{repo}
}

func (s AuthService) Login(ctx context.Context, data model.LoginRequest) (signedToken string, err error) {
	user, err := s.repo.GetOneUserByEmail(ctx, data.Email)
	if err != nil {
		return "", err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(data.Password))
	if err != nil {
		return "", bcrypt.ErrMismatchedHashAndPassword
	}
	token := jwt.New(jwt.SigningMethodHS256)
	signedToken, err = token.SignedString([]byte(user.APIKey))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func (s AuthService) AdminLogin(ctx context.Context, data model.LoginRequest) (signedToken string, err error) {
	admin, err := s.repo.GetAdminByEmail(ctx, data.Email)
	if err != nil {
		return "", err
	}
	err = bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(data.Password))
	if err != nil {
		return "", bcrypt.ErrMismatchedHashAndPassword
	}
	authToken, err := s.repo.GenerateAuthDetail(ctx, admin.ID)
	claim := jwt.MapClaims{
		"exp":      time.Now().Add(time.Hour * 3).Unix(),
		"iat":      time.Now().Unix(),
		"userID":   admin.ID,
		"role":     admin.Role.Name,
		"authUUID": authToken.AuthUUID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	signedToken, err = token.SignedString([]byte(admin.Role.Name))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func (s AuthService) Logout(ctx context.Context) error {
	s.repo.InvalidateToken(ctx, ctx.Value("userID").(uint), ctx.Value("authUUID").(string))
	return nil
}
