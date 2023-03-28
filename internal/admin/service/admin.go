package service

import (
	"context"
	"gateway/internal/admin/model"
	"gateway/internal/admin/repository"
	"log"
	"math/rand"
	"net/url"

	"golang.org/x/crypto/bcrypt"
)

type adminService struct {
	repo repository.Repo
}

func NewAdminService(adminRepo repository.Repo) AdminService {
	return &adminService{adminRepo}
}

func (svc adminService) CreateNewClient(ctx context.Context, data model.CreateClient) (model.Client, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), 14)
	if err != nil {
		log.Println(err.Error())
		return model.Client{}, err
	}
	clientId := generateClientId()
	clientSecret := generateClientSecret(clientId)
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), 14)
	if err != nil {
		log.Println(err.Error())
		return model.Client{}, err
	}
	client := model.Client{
		Name:               data.Name,
		Email:              data.Email,
		Password:           string(hashedPassword),
		ClientId:           clientId,
		HashedClientSecret: string(hashedSecret),
	}
	svc.repo.CreateNewClient(ctx, client)
	return model.Client{}, nil
}

func (svc adminService) GetAllClient(ctx context.Context, query url.Values) (clients []model.Client, err error) {
	clients, err = svc.repo.GetAllClient(ctx, nil)
	if err != nil {
		log.Println(err.Error())
		return []model.Client{}, err
	}
	return clients, nil
}

func generateClientId() string {
	runes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	clientId := make([]rune, 10)
	for i := range clientId {
		clientId[i] = runes[rand.Intn(len(runes))]
	}
	return string(clientId)
}

func generateClientSecret(clientId string) string {
	runes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.!@#$%^&*")
	secret := make([]rune, 32)
	for i := range secret {
		secret[i] = runes[rand.Intn(len(runes))]
	}
	return string(secret)
}
