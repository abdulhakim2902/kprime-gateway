package service

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/repositories"
	"gateway/internal/user/types"
	"gateway/pkg/ws"
	"os"
	"strconv"
	"strings"
	"time"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"github.com/golang-jwt/jwt"
)

type AuthService struct {
	repo *repositories.UserRepository
}

func NewAuthService(repo *repositories.UserRepository) IAuthService {
	return &AuthService{repo}
}

func (s AuthService) Login(ctx context.Context, req types.AuthRequest) (res *types.AuthResponse, user *types.User, err error) {
	user, err = s.repo.FindByAPIKeyAndSecret(ctx, req.APIKey, req.APISecret)
	if err != nil && !strings.Contains(err.Error(), "no documents in result") {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	if user == nil {
		err = errors.New("invalid credential")
		return
	}

	accessToken, refreshToken, accessTokenExp, err := GenerateToken(user.ID.Hex())
	if err != nil {
		err = errors.New("failed to generate token")
		return
	}

	res = &types.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTokenExp),
		Scope:        "connection mainaccount",
		TokenType:    "bearer",
	}

	return
}

func (s AuthService) RefreshToken(ctx context.Context, claim types.JwtClaim) (res *types.AuthResponse, user *types.User, err error) {
	user, err = s.repo.FindById(ctx, claim.UserID)
	if err != nil && !strings.Contains(err.Error(), "no documents in result") {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	if user == nil {
		err = errors.New("invalid refresh token")
		return
	}

	accessToken, refreshToken, accessTokenExp, err := GenerateToken(user.ID.Hex())
	if err != nil {
		err = errors.New("failed to generate token")
		return
	}

	res = &types.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTokenExp),
		Scope:        "connection mainaccount",
		TokenType:    "bearer",
	}

	return
}

func ClaimJWT(c *ws.Client, jwtToken string) (types.JwtClaim, error) {
	// If c not nil, check is client is authed connection
	if c != nil {
		if isAuthed, userId := c.IsAuthed(); isAuthed {
			return types.JwtClaim{
				UserID: userId,
			}, nil
		}
	}

	jwtKey := os.Getenv("JWT_KEY")

	// Second if user is not authenticated using WS client, then check JWT Token
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
	fmt.Println("claims")
	fmt.Println(claims)

	userId, ok := claims["userID"].(string)
	if !ok {
		return types.JwtClaim{}, errors.New("invalid token")
	}

	return types.JwtClaim{
		UserID: userId,
	}, nil
}

func GenerateToken(userId string) (accessToken, refreshToken string, accessTokenExp int, err error) {
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
		logs.Log.Error().Err(err).Msg("")
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
		logs.Log.Error().Err(err).Msg("")
		return
	}

	return
}
