package controller

import (
	"encoding/json"
	_authModel "gateway/internal/auth/model"
	"gateway/internal/auth/service"
	"gateway/pkg/model"
	"net/http"

	"gateway/pkg/rbac/middleware"

	cors "github.com/rs/cors/wrapper/gin"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

type authHandler struct {
	svc      service.IAuthService
	enforcer *casbin.Enforcer
}

func NewAuthHandler(r *gin.Engine, svc service.IAuthService, enforcer *casbin.Enforcer) {
	handler := &authHandler{
		svc:      svc,
		enforcer: enforcer,
	}
	r.Use(cors.AllowAll())

	authRoute := r.Group("/auth")
	authRoute.POST("/roles", middleware.Authenticate(), handler.GetRoles)
	userAuthRoute := authRoute.Group("user")
	adminAuthRoute := authRoute.Group("admin")
	userAuthRoute.POST("/login", handler.Login)
	adminAuthRoute.POST("/login", handler.AdminLogin)
}

func (h authHandler) GetRoles(r *gin.Context) {
	role, err := r.Get("role")
	if !err {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error: true,
		})
		return
	}
	roleStr, ok := role.(string)
	if !ok {
		r.JSON(http.StatusBadRequest, &model.Response{
			Error: true,
		})
		return
	}

	filteredPolicy := h.enforcer.GetFilteredPolicy(0, roleStr)

	v1V2Values := make([][]string, 0)

	for _, rule := range filteredPolicy {
		v1V2Values = append(v1V2Values, []string{rule[1], rule[2]})
	}

	r.JSON(http.StatusOK, &model.Response{
		Data: v1V2Values,
	})
	return
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
