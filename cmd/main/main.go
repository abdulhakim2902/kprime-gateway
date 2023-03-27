package main

import (
	"gateway/internal/admin/controller"
	"gateway/internal/admin/repository"
	"gateway/internal/admin/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	r := gin.New()
	dsn := "root:secret@tcp(127.0.0.1:3306)/option_exchange"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	repo := repository.NewAdminRepo(db)
	svc := service.NewAdminService(repo)
	controller.NewAdminHandler(r, svc)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
