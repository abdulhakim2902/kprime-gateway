package service

import (
	"context"
	"gateway/internal/admin/model"
	"gateway/internal/admin/repository"
)

type adminService struct {
	repo repository.Repo
}

func NewAdminService(adminRepo repository.Repo) AdminService {
	return &adminService{adminRepo}
}

func (svc adminService) CreateNewClient(ctx context.Context, data model.CreateClient) model.Client {
	return model.Client{}
}
