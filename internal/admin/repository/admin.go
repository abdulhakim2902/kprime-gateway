package repository

import (
	"context"
	"gateway/internal/admin/model"
	_adminModel "gateway/internal/admin/model"
	_userModel "gateway/internal/user/model"

	"gorm.io/gorm"
)

type adminRepo struct {
	db *gorm.DB
}

func NewAdminRepo(db *gorm.DB) IAdminRepo {
	return &adminRepo{db}
}

func (repo *adminRepo) Register(ctx context.Context, data model.Admin) (admin model.Admin, err error) {
	_ = repo.db.Create(&data)
	return admin, nil
}

func (repo *adminRepo) CreateNewClient(ctx context.Context, data _userModel.Client) (_userModel.APIKeys, error) {
	_ = repo.db.Create(&data)

	return _userModel.APIKeys{}, nil
}

func (repo *adminRepo) CreateNewRole(ctx context.Context, data _adminModel.Role) (_adminModel.ResponseRole, error) {
	_ = repo.db.Create(&data)

	return _adminModel.ResponseRole{}, nil
}

func (repo *adminRepo) CreateNewPermission(ctx context.Context, data _adminModel.Permission) (_adminModel.ResponsePermission, error) {
	_ = repo.db.Create(&data)

	return _adminModel.ResponsePermission{}, nil
}

func (repo *adminRepo) CreateNewCasbin(ctx context.Context, data _adminModel.Casbin, id int) (_adminModel.ResponseCasbin, error) {
	_ = repo.db.Create(&data)

	return _adminModel.ResponseCasbin{}, nil
}

func (repo *adminRepo) DetailRole(ctx context.Context, id int) (roles []_adminModel.Role, err error) {
	_ = repo.db.Raw("SELECT * FROM roles WHERE ID = ?", id).Scan(&roles)

	return roles, err
}

func (repo *adminRepo) UpdateRole(ctx context.Context, data _adminModel.Role, id int) (_adminModel.ResponseRole, error) {
	_ = repo.db.Where("ID = ? ", id).Updates(&data)

	return _adminModel.ResponseRole{}, nil
}

func (repo *adminRepo) UpdatePermission(ctx context.Context, data _adminModel.Permission, id int) (_adminModel.ResponsePermission, error) {
	_ = repo.db.Where("ID = ? ", id).Updates(&data)

	return _adminModel.ResponsePermission{}, nil
}

func (repo *adminRepo) DeleteRole(ctx context.Context, id int) (_adminModel.ResponseRole, error) {
	_ = repo.db.Delete(&_adminModel.Role{
		Model: gorm.Model{ID: uint(id)},
	})

	return _adminModel.ResponseRole{}, nil
}

func (repo *adminRepo) DeleteCasbin(ctx context.Context, id int) (_adminModel.ResponseCasbin, error) {
	_ = repo.db.Delete(&_adminModel.Casbin{ID: uint(id)})

	return _adminModel.ResponseCasbin{}, nil
}

func (repo *adminRepo) DeleteClient(ctx context.Context, id int) (_userModel.ResponseClient, error) {
	_ = repo.db.Delete(&_userModel.Client{ID: uint(id)})

	return _userModel.ResponseClient{}, nil
}

func (repo *adminRepo) GetAllPermission(ctx context.Context, query map[string]interface{}) (permissions []_adminModel.Permission, err error) {
	_ = repo.db.Find(&permissions)

	return permissions, err
}

func (repo *adminRepo) GetAllClient(ctx context.Context, query map[string]interface{}) (clients []_userModel.Client, err error) {
	_ = repo.db.Joins("Role").Find(&clients)

	return clients, err
}

func (repo *adminRepo) GetAllRole(ctx context.Context, query map[string]interface{}) (roles []_adminModel.Role, err error) {
	_ = repo.db.Find(&roles)

	return roles, err
}

func (repo *adminRepo) GetAllCashbin(ctx context.Context, query map[string]interface{}) (cashbins []_adminModel.Casbin, err error) {
	_ = repo.db.Find(&cashbins)

	return cashbins, err
}

func (repo *adminRepo) GetByName(ctx context.Context, name string) (admin model.Admin, err error) {
	result := repo.db.Where("name = ?", name).First(&admin)

	return admin, result.Error
}

func (repo *adminRepo) GetById(ctx context.Context, id int) (client _userModel.Client, err error) {
	_ = repo.db.Where("ID = ?", id).First(&client)

	return client, err
}

func (repo *adminRepo) UpdateClient(ctx context.Context, data _userModel.Client, id int) (_userModel.ResponseClient, error) {
	_ = repo.db.Where("ID = ? ", id).Updates(&data)

	return _userModel.ResponseClient{}, nil
}
