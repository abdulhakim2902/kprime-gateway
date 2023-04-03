package service

import (
	"context"
	"gateway/internal/admin/model"
	_adminModel "gateway/internal/admin/model"
	"gateway/internal/admin/repository"
	_userModel "gateway/internal/user/model"
	"log"
	"math/rand"
	"net/url"

	"golang.org/x/crypto/bcrypt"
)

type adminService struct {
	repo repository.IAdminRepo
}

func NewAdminService(adminRepo repository.IAdminRepo) IAdminService {
	return &adminService{adminRepo}
}

func (svc adminService) Register(ctx context.Context, data model.RegisterAdmin) (admin model.Admin, err error) {

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), 14)
	if err != nil {
		log.Println(err.Error())
		return admin, err
	}
	if err != nil {
		log.Println(err.Error())
		return admin, err
	}
	admin = model.Admin{
		Name:     data.Name,
		Email:    data.Email,
		Password: string(hashedPassword),
		RoleId:   1,
	}
	svc.repo.Register(ctx, admin)
	return admin, nil
}

func (svc adminService) CreateNewClient(ctx context.Context, data _userModel.CreateClient) (_userModel.APIKeys, error) {
	clientId := generateClientId()
	password := generateClientSecret(clientId)
	clientSecret := generateClientSecret(clientId)
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println(err.Error())
		return _userModel.APIKeys{
			APIKey:    "",
			APISecret: "",
		}, err
	}
	client := _userModel.Client{
		Name:      data.Name,
		Email:     data.Email,
		Company:   data.Company,
		Password:  string(hashedPassword),
		APIKey:    clientId,
		APISecret: string(hashedSecret),
		RoleId:    data.RoleId,
	}
	svc.repo.CreateNewClient(ctx, client)
	return _userModel.APIKeys{
		Password:  password,
		APIKey:    clientId,
		APISecret: clientSecret,
	}, nil
}

func (svc adminService) GetAllClient(ctx context.Context, query url.Values) (clients []_userModel.Client, err error) {
	clients, err = svc.repo.GetAllClient(ctx, nil)
	if err != nil {
		log.Println(err.Error())
		return []_userModel.Client{}, err
	}
	return clients, nil
}

func (svc adminService) GetAllRole(ctx context.Context, query url.Values) (roles []_adminModel.Role, err error) {
	roles, err = svc.repo.GetAllRole(ctx, nil)
	if err != nil {
		log.Println(err.Error())
		return []_adminModel.Role{}, err
	}
	return roles, nil
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
