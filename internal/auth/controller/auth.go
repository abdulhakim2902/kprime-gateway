package auth

import (
	"encoding/json"
	_authModel "gateway/internal/auth/model"
	"gateway/internal/auth/service"
	"gateway/pkg/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

type authHandler struct {
	svc service.IAuthService
}

func NewAuthHandler(r *gin.Engine, svc service.IAuthService) {
	handler := &authHandler{
		svc: svc,
	}

	r.POST("auth/login", handler.Login)
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
