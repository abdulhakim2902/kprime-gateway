package controller

import (
	"encoding/json"
	_adminModel "gateway/internal/admin/model"
	_roleModel "gateway/internal/admin/model"
	"gateway/internal/admin/service"
	_userModel "gateway/internal/user/model"
	"gateway/pkg/model"
	"gateway/pkg/rbac/middleware"
	"net/http"
	"strconv"

	cors "github.com/rs/cors/wrapper/gin"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Create a new instance of the validator
var validate = validator.New()

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
	adminRoute.GET("/role/:id", middleware.Authorize("user", "read", enforcer), handler.DetailRole)
	adminRoute.PUT("/role/:id", middleware.Authorize("user", "write", enforcer), handler.UpdateRole)
	adminRoute.POST("/role", middleware.Authorize("user", "write", enforcer), handler.CreateNewRole)
	adminRoute.DELETE("/role/:id", middleware.Authorize("user", "write", enforcer), handler.DeleteRole)

	adminRoute.POST("/request-password", middleware.Authorize("user", "write", enforcer), handler.RequestNewPassword)
	adminRoute.POST("/request-api-secret", middleware.Authorize("user", "write", enforcer), handler.RequestNewApiSecret)
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

func (h *adminHandler) UpdateRole(r *gin.Context) {
	var req _roleModel.UpdateRole
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	id, err := strconv.Atoi(r.Param("id"))
	role, err := h.svc.UpdateRole(r.Request.Context(), req, id)
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

func (h *adminHandler) DetailRole(r *gin.Context) {
	id, err := strconv.Atoi(r.Param("id"))
	roles, err := h.svc.DetailRole(r.Request.Context(), r.Request.URL.Query(), id)
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

func (h *adminHandler) RequestNewPassword(r *gin.Context) {
	// Get request body
	var req _adminModel.RequestKeyPassword
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Perform validation
	if err := validate.Struct(req); err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Call service
	_data, errJson := h.svc.RequestNewPassword(r.Request.Context(), req)
	if errJson != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: errJson.Error(),
		})
		return
	}

	r.JSON(http.StatusAccepted, &model.Response{
		Data: _data,
	})
}

func (h *adminHandler) RequestNewApiSecret(r *gin.Context) {
	// Get request body
	var req _adminModel.RequestKeyPassword
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Perform validation
	if err := validate.Struct(req); err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}

	// Call service
	_data, errJson := h.svc.RequestNewApiSecret(r.Request.Context(), req)
	if errJson != nil {
		r.JSON(http.StatusInternalServerError, &model.Response{
			Error:   true,
			Message: errJson.Error(),
		})
		return
	}

	r.JSON(http.StatusAccepted, &model.Response{
		Data: _data,
	})
}
