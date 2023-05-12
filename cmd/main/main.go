package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"
	"time"

	ordermatch "gateway/internal/fix-acceptor"
	"gateway/internal/repositories"
	"gateway/pkg/kafka/consumer"
	"gateway/pkg/mongo"
	"gateway/pkg/redis"

	// "gateway/pkg/kafka/consumer"

	_deribitCtrl "gateway/internal/deribit/controller"
	_deribitSvc "gateway/internal/deribit/service"
	_wsEngineSvc "gateway/internal/ws/engine/service"
	_wsOrderbookSvc "gateway/internal/ws/service"
	_wsSvc "gateway/internal/ws/service"

	_engSvc "gateway/internal/engine/service"
	_obSvc "gateway/internal/orderbook/service"
	_authSvc "gateway/internal/user/service"

	_wsCtrl "gateway/internal/ws/controller"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var (
	mongoConn *mongo.Database
	err       error
	rootDir   string
	mode      string
)

func init() {
	_, b, _, _ := runtime.Caller(0)
	rootDir = path.Join(b, "../../../")

	if err = godotenv.Load(path.Join(rootDir, ".env")); err != nil {
		panic(err)
	}
	mode = os.Getenv("NODE_ENV")

	mongoConn, err = mongo.InitConnection(os.Getenv("MONGO_URL"))
	if err != nil {
		panic(err)
	}
}

func main() {
	r := gin.New()
	if mode == "development" {
		gin.SetMode(gin.DebugMode)
	} else if mode == "staging" {
		gin.SetMode(gin.TestMode)
	} else if mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initiate Redis Connection Here
	redis := redis.NewRedisConnectionPool(os.Getenv("REDIS_URL"))

	_deribitSvc := _deribitSvc.NewDeribitService()
	_deribitCtrl.NewDeribitHandler(r, _deribitSvc)

	// qf
	go ordermatch.Cmd.Execute()

	// Websocket handlers
	_wsEngineSvc := _wsEngineSvc.NewwsEngineService(redis)

	userRepo := repositories.NewUserRepository(mongoConn)
	orderRepo := repositories.NewOrderRepository(mongoConn)
	tradeRepo := repositories.NewTradeRepository(mongoConn)
	rawPriceRepo := repositories.NewRawPriceRepository(mongoConn)
	settlementPriceRepo := repositories.NewSettlementPriceRepository(mongoConn)

	_wsAuthSvc := _authSvc.NewAuthService(userRepo)
	_wsOrderbookSvc := _wsOrderbookSvc.NewWSOrderbookService(redis, orderRepo, tradeRepo, rawPriceRepo, settlementPriceRepo)
	_wsOrderSvc := _wsSvc.NewWSOrderService(redis, orderRepo)
	_wsTradeSvc := _wsSvc.NewWSTradeService(redis, tradeRepo)

	_wsCtrl.NewWebsocketHandler(
		r,
		_wsAuthSvc,
		_deribitSvc,
		_wsOrderbookSvc,
		_wsEngineSvc,
		_wsOrderSvc,
		_wsTradeSvc,
		userRepo,
	)

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
	_engSvc := _engSvc.NewEngineHandler(r, redis, tradeRepo, _wsOrderbookSvc)

	// kafka listener
	consumer.KafkaConsumer(orderRepo, _engSvc, _obSvc, _wsOrderSvc, _wsTradeSvc)

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
