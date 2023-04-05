package main

import (
	"context"
	"fmt"
	"gateway/database/seeder/seeds"
	"gateway/internal/admin/controller"
	_adminModel "gateway/internal/admin/model"
	"gateway/internal/admin/repository"
	"gateway/internal/admin/service"
	_authModel "gateway/internal/auth/model"
	"gateway/internal/user/model"
	"gateway/pkg/kafka/consumer"
	"gateway/pkg/redis"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"
	"time"

	// "gateway/pkg/kafka/consumer"

	_authCtrl "gateway/internal/auth/controller"
	_authRepo "gateway/internal/auth/repository"
	_authSvc "gateway/internal/auth/service"

	_deribitCtrl "gateway/internal/deribit/controller"
	_deribitSvc "gateway/internal/deribit/service"
	_wsOrderbookSvc "gateway/internal/ws/service"

	_obSvc "gateway/internal/orderbook/service"

	_wsCtrl "gateway/internal/ws/controller"

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
	if os.Getenv("NODE_ENV") == "development" {
		gin.SetMode(gin.DebugMode)
	} else if os.Getenv("NODE_ENV") == "staging" {
		gin.SetMode(gin.TestMode)
	} else if os.Getenv("NODE_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

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
	if gin.Mode() != gin.ReleaseMode {
		db.AutoMigrate(&model.Client{}, &_adminModel.Admin{}, &_adminModel.Role{}, &_authModel.TokenAuth{})
	}

	// Seed Database
	for _, seed := range seeds.All() {
		if err := seed.Run(db); err != nil {
			log.Fatalf("Running seed '%s', failed with error:\n%s", seed.Name, err)
		}
	}
	setupRBAC(enforcer)

	// Initiate Redis Connection Here
	redis := redis.NewRedisConnection(os.Getenv("REDIS_URL"))

	adminRepo := repository.NewAdminRepo(db)
	adminSvc := service.NewAdminService(adminRepo)
	controller.NewAdminHandler(r, adminSvc, enforcer)

	authRepo := _authRepo.NewAuthRepo(db)
	authSvc := _authSvc.NewAuthService(authRepo)
	_authCtrl.NewAuthHandler(r, authSvc, enforcer)

	_deribitSvc := _deribitSvc.NewDeribitService()
	_deribitCtrl.NewDeribitHandler(r, _deribitSvc)

	//qf
	go ordermatch.Cmd.Execute()
	_wsOrderbookSvc := _wsOrderbookSvc.NewwsOrderbookService()

	_wsCtrl.NewWebsocketHandler(r, authSvc, _deribitSvc, _wsOrderbookSvc)

	fmt.Printf("Server is running on %s \n", os.Getenv("PORT"))

	srv := &http.Server{
		Addr:    ":" + os.Getenv("PORT"),
		Handler: r,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	_obSvc := _obSvc.NewOrderbookHandler(r, redis)

	//kafka listener
	consumer.KafkaConsumer(_obSvc)

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
