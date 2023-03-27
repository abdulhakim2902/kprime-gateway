package controller

import (
	"gateway/internal/admin/service"
	"gateway/pkg/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

type adminHandler struct {
	svc service.AdminService
}

func NewAdminHandler(r *gin.Engine, svc service.AdminService) {
	handler := &adminHandler{
		svc: svc,
	}

	r.POST("admin/client/new", handler.CreateNewClient)
	r.GET("admin/client", handler.GetAllClient)
}

func (h *adminHandler) CreateNewClient(r *gin.Context) {

}

func (h *adminHandler) GetAllClient(r *gin.Context) {
	r.JSON(http.StatusOK, &model.Response{
		Data: "hola",
	})
}
