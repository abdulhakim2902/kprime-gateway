package controller

import (
	"encoding/json"
	_authModel "gateway/internal/auth/model"
	"gateway/internal/auth/service"
	"gateway/pkg/model"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

type authHandler struct {
	svc service.IAuthService
}

func NewAuthHandler(r *gin.Engine, svc service.IAuthService, enforcer *casbin.Enforcer) {
	handler := &authHandler{
		svc: svc,
	}

	authRoute := r.Group("/auth")
	userAuthRoute := authRoute.Group("user")
	adminAuthRoute := authRoute.Group("admin")
	userAuthRoute.POST("/login", handler.Login)
	adminAuthRoute.POST("/login", handler.AdminLogin)
}

func (h authHandler) Login(r *gin.Context) {
	var req _authModel.LoginRequest
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	token, err := h.svc.Login(r.Request.Context(), req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	r.JSON(http.StatusOK, &model.Response{
		Data: token,
	})
	return
}

func (h authHandler) AdminLogin(r *gin.Context) {
	var req _authModel.LoginRequest
	err := json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	token, err := h.svc.AdminLogin(r.Request.Context(), req)
	if err != nil {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error:   true,
			Message: err.Error(),
		})
		return
	}
	r.JSON(http.StatusOK, &model.Response{
		Data: token,
	})
	return
}
