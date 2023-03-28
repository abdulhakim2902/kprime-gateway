package main

import (
	"gateway/internal/admin/controller"
	"gateway/internal/admin/model"
	"gateway/internal/admin/repository"
	"gateway/internal/admin/service"
	"net/http"

	_authCtrl "gateway/internal/auth/controller"
	_authRepo "gateway/internal/auth/repository"
	_authSvc "gateway/internal/auth/service"

	_deribitCtrl "gateway/internal/deribit/controller"
	_deribitSvc "gateway/internal/deribit/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	r := gin.New()
	dsn := "root:@tcp(127.0.0.1:3306)/option_exchange?parseTime=true"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	db.AutoMigrate(&model.Client{})
	adminRepo := repository.NewAdminRepo(db)
	adminSvc := service.NewAdminService(adminRepo)
	controller.NewAdminHandler(r, adminSvc)

	authRepo := _authRepo.NewAuthRepo(db)
	authSvc := _authSvc.NewAuthService(authRepo)
	_authCtrl.NewAuthHandler(r, authSvc)

	_deribitSvc := _deribitSvc.NewDeribitService()
	_deribitCtrl.NewDeribitHandler(r, _deribitSvc)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
