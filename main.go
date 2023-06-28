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
	"strconv"
	"syscall"
	"time"

	"gateway/cmd/server"
	ordermatch "gateway/internal/fix-acceptor"
	"gateway/internal/repositories"
	"gateway/pkg/collector"
	"gateway/pkg/kafka/consumer"
	"gateway/pkg/memdb"
	"gateway/pkg/mongo"
	"gateway/pkg/redis"
	"gateway/pkg/utils"

	_deribitCtrl "gateway/internal/deribit/controller"
	_deribitSvc "gateway/internal/deribit/service"
	_engSvc "gateway/internal/engine/service"
	_obSvc "gateway/internal/orderbook/service"
	_userSvc "gateway/internal/user/service"
	_wsEngineSvc "gateway/internal/ws/engine/service"
	_wsOrderbookSvc "gateway/internal/ws/service"
	_wsSvc "gateway/internal/ws/service"

	_wsCtrl "gateway/internal/ws/controller"

	memory "gateway/datasources/memdb"
	docs "gateway/docs"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/commons/metrics"
	memoryDb "git.devucc.name/dependencies/utilities/repository/memdb"
	"git.devucc.name/dependencies/utilities/repository/mongodb"
	"git.devucc.name/dependencies/utilities/schema"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/ulule/limiter/v3"
	limiterMem "github.com/ulule/limiter/v3/drivers/store/memory"

	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware
)

var (
	engine    *gin.Engine
	mongoConn *mongo.Database
	redisConn *redis.RedisConnectionPool
	memDb     *memdb.Schemas

	err     error
	rootDir string
)

func init() {
	_, b, _, _ := runtime.Caller(0)
	rootDir = path.Join(b, "../")
	fmt.Println(rootDir)
	docs.SwaggerInfo.BasePath = "/api/v2"
	if err = godotenv.Load(path.Join(rootDir, ".env")); err != nil {
		panic(err)
	}

	// Gin Engine
	engine = gin.New()

	mode := os.Getenv("NODE_ENV")
	if mode == "development" {
		gin.SetMode(gin.DebugMode)
	} else if mode == "staging" {
		gin.SetMode(gin.TestMode)
	} else if mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Logger
	utils.InitLogger()

	// Init Mongo Connection
	mongoConn, err = mongo.InitConnection(os.Getenv("MONGO_URL"))
	if err != nil {
		logs.Log.Fatal().Err(err).Msg("failed to connect with mongo")
	}

	// Initiate Redis Connection
	redisConn = redis.NewRedisConnectionPool(os.Getenv("REDIS_URL"))

	// Initialize MemoryDB schemas
	err := memory.InitSchema(schema.Schema)
	if err != nil {
		logs.Log.Fatal().Err(err).Msg("failed to initialize memory schemas")
	}

	mongoRepo := mongodb.NewRepositories(mongoConn)
	memRepo := memoryDb.NewRepositories(memory.Database)
	server.InitializeData(mongoRepo, memRepo)
}

// @title           Gateway Internal API
// @version         2.0
// @description     This is used for internal service

// @host      localhost:8080
// @BasePath  /api/v2

// @securityDefinitions.basic  BasicAuth
func main() {
	// qf
	store := limiterMem.NewStore()
	p, ok := os.LookupEnv("RATE_LIMITER_DURATION")
	if !ok {
		p = "1"
	}
	l, ok := os.LookupEnv("RATE_LIMITER_MAX_REQUESTS")
	if !ok {
		l = "5"
	}
	period, _ := strconv.ParseInt(p, 10, 64)
	limit, _ := strconv.ParseInt(l, 10, 64)

	limiter := &limiter.Limiter{
		Rate: limiter.Rate{
			Period: time.Duration(period) * time.Second,
			Limit:  limit,
		},
		Store: store,
	}
	// engine.Use(middleware.RateLimiter(limiter))

	memoryDb, err := memdb.InitSchemas()
	if err != nil {
		logs.Log.Fatal().Err(err).Msg("failed to initialize memory schemas")
	}

	// Websocket handlers
	_wsEngineSvc := _wsEngineSvc.NewwsEngineService(redisConn)

	userRepo := repositories.NewUserRepository(mongoConn)
	orderRepo := repositories.NewOrderRepository(mongoConn)
	tradeRepo := repositories.NewTradeRepository(mongoConn)
	rawPriceRepo := repositories.NewRawPriceRepository(mongoConn)
	settlementPriceRepo := repositories.NewSettlementPriceRepository(mongoConn)

	_authSvc := _userSvc.NewAuthService(userRepo, memoryDb)
	_wsOrderbookSvc := _wsOrderbookSvc.NewWSOrderbookService(
		redisConn,
		orderRepo,
		tradeRepo,
		rawPriceRepo,
		settlementPriceRepo,
	)
	_wsOrderSvc := _wsSvc.NewWSOrderService(redisConn, orderRepo)
	_wsTradeSvc := _wsSvc.NewWSTradeService(redisConn, tradeRepo)
	_wsRawPriceSvc := _wsSvc.NewWSRawPriceService(redisConn, rawPriceRepo)
	_wsUserBalanceSvc := _wsSvc.NewWSUserBalanceService()

	_userSvc := _userSvc.NewUserService(engine, userRepo, memoryDb)

	_userSvc.SyncMemDB(context.TODO(), nil)

	_deribitSvc := _deribitSvc.NewDeribitService(
		redisConn,
		memoryDb,
		tradeRepo,
		orderRepo,
		rawPriceRepo,
		settlementPriceRepo,
	)

	go ordermatch.Execute(_deribitSvc)

	_deribitCtrl.NewDeribitHandler(engine, _deribitSvc, _authSvc, userRepo, memoryDb)
	_wsCtrl.NewWebsocketHandler(
		engine,
		_authSvc,
		_deribitSvc,
		_wsOrderbookSvc,
		_wsEngineSvc,
		_wsOrderSvc,
		_wsTradeSvc,
		_wsRawPriceSvc,
		_wsUserBalanceSvc,
		userRepo,
		limiter,
	)

	fmt.Printf("Server is running on %s \n", os.Getenv("PORT"))
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	srv := &http.Server{
		Addr:    ":" + os.Getenv("PORT"),
		Handler: engine,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	go func() {
		// metrics connections
		m := metrics.NewMetrics()
		m.RegisterCollector(
			collector.IncomingCounter,
			collector.SuccessCounter,
			collector.ValidationCounter,
			collector.ErrorCounter,
			collector.OutgoingKafkaCounter,
			collector.IncomingKafkaCounter,
			collector.RequestDurationHistogram,
			collector.KafkaDurationHistogram,
		)

		if err := m.Serve(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}

	}()

	_obSvc := _obSvc.NewOrderbookHandler(engine, redisConn, _wsOrderbookSvc)
	_engSvc := _engSvc.NewEngineHandler(engine, redisConn, tradeRepo, _wsOrderbookSvc)

	// kafka listener
	consumer.KafkaConsumer(orderRepo, _engSvc, _obSvc, _wsOrderSvc, _wsTradeSvc, _wsRawPriceSvc)

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
