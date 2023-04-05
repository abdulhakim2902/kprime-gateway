package controller

import (
	"encoding/json"
	_adminModel "gateway/internal/admin/model"
	"gateway/internal/admin/service"
	_roleModel "gateway/internal/role/model"
	_userModel "gateway/internal/user/model"
	"gateway/pkg/model"
	"gateway/pkg/rbac/middleware"
	"net/http"
	"strconv"

	cors "github.com/rs/cors/wrapper/gin"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

type adminHandler struct {
	svc service.IAdminService
}

func NewAdminHandler(r *gin.Engine, svc service.IAdminService, enforcer *casbin.Enforcer) {
	handler := &adminHandler{
		svc: svc,
	}
	r.Use(cors.AllowAll())
	adminRoute := r.Group("/admin")

	adminRoute.POST("/register", handler.Register)

	adminRoute.GET("/client", middleware.Authorize("user", "read", enforcer), handler.GetAllClient)
	adminRoute.POST("/client", middleware.Authorize("user", "write", enforcer), handler.CreateNewClient)
	adminRoute.DELETE("/client/:id", middleware.Authorize("user", "write", enforcer), handler.DeleteClient)

	adminRoute.GET("/role", middleware.Authorize("user", "read", enforcer), handler.GetAllRole)
	adminRoute.POST("/role", middleware.Authorize("user", "write", enforcer), handler.CreateNewRole)
	adminRoute.DELETE("/role/:id", middleware.Authorize("user", "write", enforcer), handler.DeleteRole)
}

func (h *adminHandler) Register(r *gin.Context) {
	var req _adminModel.RegisterAdmin
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	admin, err := h.svc.Register(r.Request.Context(), req)
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	r.JSON(http.StatusAccepted, &model.Response{
		Data: admin,
	})
}

func (h *adminHandler) CreateNewClient(r *gin.Context) {
	var req _userModel.CreateClient
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	client, err := h.svc.CreateNewClient(r.Request.Context(), req)
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	r.JSON(http.StatusAccepted, &model.Response{
		Data: client,
	})
	return
}

func (h *adminHandler) CreateNewRole(r *gin.Context) {
	var req _roleModel.CreateRole
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	role, err := h.svc.CreateNewRole(r.Request.Context(), req)
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	r.JSON(http.StatusAccepted, &model.Response{
		Data: role,
	})
	return
}

func (h *adminHandler) DeleteRole(r *gin.Context) {
	id, err := strconv.Atoi(r.Param("id"))
	role, err := h.svc.DeleteRole(r.Request.Context(), id)
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	r.JSON(http.StatusAccepted, &model.Response{
		Data: role,
	})
	return
}

func (h *adminHandler) DeleteClient(r *gin.Context) {
	id, err := strconv.Atoi(r.Param("id"))
	client, err := h.svc.DeleteClient(r.Request.Context(), id)
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	r.JSON(http.StatusAccepted, &model.Response{
		Data: client,
	})
	return
}

func (h *adminHandler) GetAllClient(r *gin.Context) {
	clients, err := h.svc.GetAllClient(r.Request.Context(), r.Request.URL.Query())
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	r.JSON(http.StatusOK, &model.Response{
		Data: clients,
	})
	return
}

func (h *adminHandler) GetAllRole(r *gin.Context) {
	roles, err := h.svc.GetAllRole(r.Request.Context(), r.Request.URL.Query())
	if err != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	r.JSON(http.StatusOK, &model.Response{
		Data: roles,
	})
	return
}
