package main

import (
	"context"
	"gateway/internal/admin/controller"
	_adminModel "gateway/internal/admin/model"
	"gateway/internal/admin/repository"
	"gateway/internal/admin/service"
	"gateway/internal/user/model"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_authCtrl "gateway/internal/auth/controller"
	_authRepo "gateway/internal/auth/repository"
	_authSvc "gateway/internal/auth/service"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	err := godotenv.Load("../../.env")
	if err != nil {
		panic(err)
	}
	r := gin.New()
	dsn := os.Getenv("DB_CONNECTION")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		panic("failed to load rbac config")
	}

	enforcer, err := casbin.NewEnforcer("../../pkg/rbac/config/model.conf", adapter)
	if err != nil {
		panic(err)
	}

	//dev only
	db.AutoMigrate(&model.Client{}, &_adminModel.Admin{}, &_adminModel.Role{})
	
	adminRepo := repository.NewAdminRepo(db)
	adminSvc := service.NewAdminService(adminRepo)
	controller.NewAdminHandler(r, adminSvc, enforcer)

	authRepo := _authRepo.NewAuthRepo(db)
	authSvc := _authSvc.NewAuthService(authRepo)
	_authCtrl.NewAuthHandler(r, authSvc, enforcer)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	// kill (no param) default send syscanll.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	// catching ctx.Done(). timeout of 5 seconds.
	select {
	case <-ctx.Done():
		log.Println("timeout of 5 seconds.")
	}
	log.Println("Server exiting")
}
