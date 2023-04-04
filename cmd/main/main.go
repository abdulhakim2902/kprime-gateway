package main

import (
	"context"
	"fmt"
	"gateway/internal/admin/controller"
	_adminModel "gateway/internal/admin/model"
	"gateway/internal/admin/repository"
	"gateway/internal/admin/service"
	_authModel "gateway/internal/auth/model"
	"gateway/internal/user/model"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"
	"time"

	"gateway/pkg/kafka/consumer/order"
	"gateway/pkg/kafka/consumer/orderbook"
	"gateway/pkg/kafka/consumer/trade"

	_authCtrl "gateway/internal/auth/controller"
	_authRepo "gateway/internal/auth/repository"
	_authSvc "gateway/internal/auth/service"

	_deribitCtrl "gateway/internal/deribit/controller"
	_deribitSvc "gateway/internal/deribit/service"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	_, b, _, _ := runtime.Caller(0)
	rootDir := path.Join(b, "../../../")
	fmt.Println(b)
	fmt.Println(rootDir)
	err := godotenv.Load(path.Join(rootDir, ".env"))
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

	enforcer, err := casbin.NewEnforcer(path.Join(rootDir, "pkg/rbac/config/model.conf"), adapter)
	if err != nil {
		panic(err)
	}

	//dev only
	db.AutoMigrate(&model.Client{}, &_adminModel.Admin{}, &_adminModel.Role{}, &_authModel.TokenAuth{})
	setupRBAC(enforcer)

	adminRepo := repository.NewAdminRepo(db)
	adminSvc := service.NewAdminService(adminRepo)
	controller.NewAdminHandler(r, adminSvc, enforcer)

	authRepo := _authRepo.NewAuthRepo(db)
	authSvc := _authSvc.NewAuthService(authRepo)
	_authCtrl.NewAuthHandler(r, authSvc, enforcer)

	_deribitSvc := _deribitSvc.NewDeribitService()
	_deribitCtrl.NewDeribitHandler(r, _deribitSvc)

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

	//kafka listener
	order.ConsumeOrder()
	trade.ConsumeTrade()
	orderbook.ConsumeOrderbook()

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

func setupRBAC(enforcer *casbin.Enforcer) {
	if hasPolicy := enforcer.HasPolicy("admin", "user", "read"); !hasPolicy {
		enforcer.AddPolicy("admin", "user", "read")
	}
	if hasPolicy := enforcer.HasPolicy("admin", "user", "write"); !hasPolicy {
		enforcer.AddPolicy("admin", "user", "write")
	}
	if hasPolicy := enforcer.HasPolicy("admin", "user", "delete"); !hasPolicy {
		enforcer.AddPolicy("admin", "user", "delete")
	}

	// Role: admin
	if hasPolicy := enforcer.HasPolicy("admin", "role", "read"); !hasPolicy {
		enforcer.AddPolicy("admin", "role", "read")
	}
	if hasPolicy := enforcer.HasPolicy("admin", "role", "write"); !hasPolicy {
		enforcer.AddPolicy("admin", "role", "write")
	}
	if hasPolicy := enforcer.HasPolicy("admin", "role", "delete"); !hasPolicy {
		enforcer.AddPolicy("admin", "role", "delete")
	}

	// Role: market_maker
	if hasPolicy := enforcer.HasPolicy("market_maker", "trading", "buy"); !hasPolicy {
		enforcer.AddPolicy("market_maker", "trading", "buy")
	}
	if hasPolicy := enforcer.HasPolicy("market_maker", "trading", "sell"); !hasPolicy {
		enforcer.AddPolicy("market_maker", "trading", "sell")
	}
	if hasPolicy := enforcer.HasPolicy("market_maker", "instrument", "write"); !hasPolicy {
		enforcer.AddPolicy("market_maker", "instrument", "write")
	}

	// Role: client/user
	if hasPolicy := enforcer.HasPolicy("user", "trading", "sell"); !hasPolicy {
		enforcer.AddPolicy("user", "trading", "sell")
	}
	if hasPolicy := enforcer.HasPolicy("user", "trading", "buy"); !hasPolicy {
		enforcer.AddPolicy("user", "trading", "buy")
	}
}
