package controller

import "github.com/gin-gonic/gin"

type IAuthHandler interface {
	Login(r *gin.Context)
	AdminLogin(r *gin.Context)
}
