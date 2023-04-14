package service

import (
	"context"
	"gateway/internal/admin/model"
	_adminModel "gateway/internal/admin/model"
	_roleModel "gateway/internal/admin/model"
	"gateway/internal/admin/repository"
	_userModel "gateway/internal/user/model"
	"log"
	"math/rand"
	"net/url"

	"gateway/pkg/email"

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

	email.SendMail(data.Email, password, clientId, clientSecret)

	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 14)

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

func (svc adminService) CreateNewRole(ctx context.Context, data _roleModel.CreateRole) (_roleModel.ResponseRole, error) {
	role := _roleModel.Role{
		Name: data.Name,
		Data: data.Data,
	}
	svc.repo.CreateNewRole(ctx, role)
	return _roleModel.ResponseRole{
		Response: "Create Success!",
	}, nil
}

func (svc adminService) DeleteRole(ctx context.Context, id int) (_roleModel.ResponseRole, error) {
	svc.repo.DeleteRole(ctx, id)
	return _roleModel.ResponseRole{
		Response: "Delete Success!",
	}, nil
}

func (svc adminService) DetailRole(ctx context.Context, query url.Values, id int) (roles []_adminModel.Role, err error) {
	roles, err = svc.repo.DetailRole(ctx, id)
	if err != nil {
		log.Println(err.Error())
		return []_adminModel.Role{}, err
	}
	return roles, nil
}

func (svc adminService) UpdateRole(ctx context.Context, data _roleModel.UpdateRole, id int) (_roleModel.ResponseRole, error) {
	role := _roleModel.Role{
		Name: data.Name,
		Data: data.Data,
	}
	svc.repo.UpdateRole(ctx, role, id)
	return _roleModel.ResponseRole{
		Response: "Update Success!",
	}, nil
}

func (svc adminService) DeleteClient(ctx context.Context, id int) (_userModel.ResponseClient, error) {
	svc.repo.DeleteClient(ctx, id)
	return _userModel.ResponseClient{
		Response: "Delete Success!",
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

func (svc adminService) RequestNewPassword(ctx context.Context, data model.RequestKeyPassword) (client interface{}, err error) {
	//get client id from request
	_clientId := data.Id

	//select client by _clientId
	_client, err := svc.repo.GetById(ctx, _clientId)
	if err != nil {
		log.Println(err.Error())
		return _userModel.Client{}, err
	}

	//get api_key
	clientApiKey := _client.APIKey

	//generate new password
	password := generateClientSecret(clientApiKey)

	//send email
	email.SendMail(_client.Email, password, "", "")

	//hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		log.Println(err.Error())
		return _userModel.APIKeys{
			APIKey:    "",
			APISecret: "",
		}, err
	}

	//update client password
	updateClient := _userModel.Client{
		Name:      _client.Name,
		Email:     _client.Email,
		Company:   _client.Company,
		Password:  string(hashedPassword),
		APIKey:    _client.APIKey,
		APISecret: _client.APISecret,
		RoleId:    _client.RoleId,
	}
	svc.repo.UpdateClient(ctx, updateClient, _clientId)

	//return api key and api secret
	return _userModel.APIKeys{
		Password:  password,
		APIKey:    _client.APIKey,
		APISecret: _client.APISecret,
	}, nil
}

func (svc adminService) RequestNewApiSecret(ctx context.Context, data model.RequestKeyPassword) (client interface{}, err error) {
	//generate new api key
	clientApiKey := generateClientId()
	clientSecret := generateClientSecret(clientApiKey)

	//get client id from request
	_clientId := data.Id

	//select client by _clientId
	_client, err := svc.repo.GetById(ctx, _clientId)
	if err != nil {
		log.Println(err.Error())
		return _userModel.Client{}, err
	}

	//send email
	email.SendMail(_client.Email, "", clientApiKey, clientSecret)

	//hash api secret
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		log.Println(err.Error())
		return _userModel.APIKeys{
			APIKey:    "",
			APISecret: "",
		}, err
	}

	//update client password
	updateClient := _userModel.Client{
		Name:      _client.Name,
		Email:     _client.Email,
		Company:   _client.Company,
		Password:  _client.Password,
		APIKey:    clientApiKey,
		APISecret: string(hashedSecret),
		RoleId:    _client.RoleId,
	}
	svc.repo.UpdateClient(ctx, updateClient, _clientId)

	//return api key and api secret
	return _userModel.APIKeys{
		Password:  _client.Password,
		APIKey:    clientApiKey,
		APISecret: clientSecret,
	}, nil
}
