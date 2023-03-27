package controller

import (
	"encoding/json"
	_adminModel "gateway/internal/admin/model"
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

	r.POST("admin/client", handler.CreateNewClient)
	r.GET("admin/client", handler.GetAllClient)
}

func (h *adminHandler) CreateNewClient(r *gin.Context) {
	var req _adminModel.CreateClient
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
	}
	client, err := h.svc.CreateNewClient(r.Request.Context(), req)
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
	}
	r.JSON(http.StatusAccepted, &model.Response{
		Data: client,
	})
}

func (h *adminHandler) GetAllClient(r *gin.Context) {
	clients, err := h.svc.GetAllClient(r.Request.Context(), r.Request.URL.Query())
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
	}
	r.JSON(http.StatusOK, &model.Response{
		Data: clients,
	})
}
