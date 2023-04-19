package service

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/auth/model"
	"gateway/internal/auth/repository"
	"os"
	"strconv"
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

	authToken, err := s.repo.GenerateAuthDetail(ctx, user.ID)
	claim := jwt.MapClaims{
		"exp":      time.Now().Add(time.Hour * 3).Unix(),
		"iat":      time.Now().Unix(),
		"userID":   user.ID,
		"role":     user.Role.Name,
		"authUUID": authToken.AuthUUID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)

	jwtKey := os.Getenv("JWT_KEY")
	signedToken, err = token.SignedString([]byte(jwtKey))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func (s AuthService) AdminLogin(ctx context.Context, data model.LoginRequest) (signedToken string, dataId uint, err error) {
	admin, err := s.repo.GetAdminByEmail(ctx, data.Email)
	if err != nil {
		return "", 0, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(data.Password))
	if err != nil {
		return "", 0, bcrypt.ErrMismatchedHashAndPassword
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

	jwtKey := os.Getenv("JWT_KEY")
	signedToken, err = token.SignedString([]byte(jwtKey))
	if err != nil {
		return "", 0, err
	}
	return signedToken, admin.ID, nil
}

func (s AuthService) APILogin(ctx context.Context, data model.APILoginRequest) (signedToken string, err error) {
	admin, err := s.repo.GetOneUserByAPIKey(ctx, data.APIKey)
	if err != nil {
		return "", err
	}
	err = bcrypt.CompareHashAndPassword([]byte(admin.APISecret), []byte(data.APISecret))
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

	jwtKey := os.Getenv("JWT_KEY")
	signedToken, err = token.SignedString([]byte(jwtKey))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func (s AuthService) Logout(ctx context.Context) error {
	s.repo.InvalidateToken(ctx, ctx.Value("userID").(uint), ctx.Value("authUUID").(string))
	return nil
}

func (s AuthService) JWTCheck(jwtToken string) (model.JWTData, error) {
	jwtKey := os.Getenv("JWT_KEY")
	tokenString := jwtToken
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(jwtKey), nil
	})

	if err != nil || !token.Valid {
		return model.JWTData{}, errors.New("Invalid token")
	}

	claims := token.Claims.(jwt.MapClaims)
	fmt.Println(claims["userID"])

	userIdFloat, ok := claims["userID"].(float64)
	if !ok {
		return model.JWTData{}, errors.New("Error: val is not a string")
	}

	userIDStr := strconv.FormatFloat(userIdFloat, 'f', 0, 64)

	return model.JWTData{
		UserID: userIDStr,
	}, nil
}
