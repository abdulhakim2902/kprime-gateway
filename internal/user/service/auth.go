package service

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/repositories"
	"gateway/internal/user/types"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

type AuthService struct {
	repo *repositories.UserRepository
}

func NewAuthService(repo *repositories.UserRepository) IAuthService {
	return &AuthService{repo}
}

func (s AuthService) Login(ctx context.Context, req types.AuthRequest) (res *types.AuthResponse, err error) {
	var user *types.User
	user, err = s.repo.FindByAPIKeyAndSecret(ctx, req.APIKey, req.APISecret)
	if err != nil && !strings.Contains(err.Error(), "no documents in result") {
		fmt.Printf("err:%+v\n", err)
		return
	}

	if user == nil {
		return nil, errors.New("invalid credential")
	}

	accessToken, refreshToken, accessTokenExp, err := s.generateToken(user.ID.Hex())
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &types.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTokenExp),
		Scope:        "connection mainaccount",
		TokenType:    "bearer",
	}, nil
}

func (s AuthService) RefreshToken(ctx context.Context, claim types.JwtClaim) (res *types.AuthResponse, err error) {
	var user *types.User
	user, err = s.repo.FindById(ctx, claim.UserID)
	if err != nil && !strings.Contains(err.Error(), "no documents in result") {
		fmt.Printf("err:%+v\n", err)
		return
	}

	if user == nil {
		return nil, errors.New("invalid refresh token")
	}

	accessToken, refreshToken, accessTokenExp, err := s.generateToken(user.ID.Hex())
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &types.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTokenExp),
		Scope:        "connection mainaccount",
		TokenType:    "bearer",
	}, nil
}

func (s AuthService) ClaimJWT(jwtToken string) (types.JwtClaim, error) {
	jwtKey := os.Getenv("JWT_KEY")

	token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(jwtKey), nil
	})

	if err != nil || !token.Valid {
		if strings.Contains(err.Error(), "Token is expired") {
			return types.JwtClaim{}, errors.New("token is expired")
		}
		return types.JwtClaim{}, errors.New("invalid token")
	}

	claims := token.Claims.(jwt.MapClaims)

	userId, ok := claims["userID"].(string)
	if !ok {
		return types.JwtClaim{}, errors.New("invalid token")
	}

	return types.JwtClaim{
		UserID: userId,
	}, nil
}

func (s AuthService) generateToken(userId string) (accessToken, refreshToken string, accessTokenExp int, err error) {
	// JWT Secret
	jwtKey := os.Getenv("JWT_KEY")

	// Access Token
	accessTokenExp, err = strconv.Atoi(os.Getenv("JWT_REMEMBER_TOKEN_EXPIRE"))
	accessTokenClaim := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp":    time.Now().Add(time.Second * time.Duration(accessTokenExp)).Unix(),
		"iat":    time.Now().Unix(),
		"userID": userId,
	})

	accessToken, err = accessTokenClaim.SignedString([]byte(jwtKey))
	if err != nil {
		fmt.Printf("err:%+v\n", err)
		return
	}

	// Refresh Token
	var refreshTokenExp int
	refreshTokenExp, err = strconv.Atoi(os.Getenv("JWT_REMEMBER_REFRESH_TOKEN_EXPIRE"))
	refreshTokenClaim := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp":    time.Now().Add(time.Second * time.Duration(refreshTokenExp)).Unix(),
		"iat":    time.Now().Unix(),
		"userID": userId,
	})

	refreshToken, err = refreshTokenClaim.SignedString([]byte(jwtKey))
	if err != nil {
		fmt.Printf("err:%+v\n", err)
		return
	}

	return
}
