package controller

import (
	"errors"
	"gateway/pkg/middleware"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"

	"gateway/internal/repositories"
	userType "gateway/internal/user/types"

	deribitService "gateway/internal/deribit/service"
	authService "gateway/internal/user/service"
	engService "gateway/internal/ws/engine/service"
	wsService "gateway/internal/ws/service"

	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"
	cors "github.com/rs/cors/wrapper/gin"
	"github.com/ulule/limiter/v3"

	"gateway/pkg/ws"

	"github.com/gin-gonic/gin"
)

type wsHandler struct {
	authSvc          authService.IAuthService
	deribitSvc       deribitService.IDeribitService
	wsOBSvc          wsService.IwsOrderbookService
	wsOSvc           wsService.IwsOrderService
	wsEngSvc         engService.IwsEngineService
	wsTradeSvc       wsService.IwsTradeService
	wsRawPriceSvc    wsService.IwsRawPriceService
	wsUserBalanceSvc wsService.IwsUserBalanceService

	userRepo *repositories.UserRepository
}

func NewWebsocketHandler(
	r *gin.Engine,
	authSvc authService.IAuthService,
	deribitSvc deribitService.IDeribitService,
	wsOBSvc wsService.IwsOrderbookService,
	wsEngSvc engService.IwsEngineService,
	wsOSvc wsService.IwsOrderService,
	wsTradeSvc wsService.IwsTradeService,
	wsRawPriceSvc wsService.IwsRawPriceService,
	wsUserBalanceSvc wsService.IwsUserBalanceService,
	userRepo *repositories.UserRepository,
	limiter *limiter.Limiter,
) {
	handler := &wsHandler{
		authSvc:          authSvc,
		deribitSvc:       deribitSvc,
		wsOBSvc:          wsOBSvc,
		wsEngSvc:         wsEngSvc,
		wsOSvc:           wsOSvc,
		wsTradeSvc:       wsTradeSvc,
		wsRawPriceSvc:    wsRawPriceSvc,
		wsUserBalanceSvc: wsUserBalanceSvc,
		userRepo:         userRepo,
	}
	r.Use(cors.AllowAll())
	r.Use(middleware.RateLimiter(limiter))
	r.GET("/ws/api/v2", ws.ConnectionEndpoint)

	middleware.SetupWSLimiter(limiter)

	handler.RegisterPrivate()
	handler.RegisterPublic()

}

func requestHelper(
	msgID uint64,
	method string,
	accessToken *string,
	c *ws.Client,
) (claim userType.JwtClaim, connKey string, reason *validation_reason.ValidationReason, err error) {
	key := utils.GetKeyFromIdUserID(msgID, "")
	if isDuplicateConnection := protocol.RegisterProtocolRequest(
		key, protocol.ProtocolRequest{WS: c, Protocol: protocol.Websocket, Method: method},
	); isDuplicateConnection {
		validation := validation_reason.DUPLICATED_REQUEST_ID
		reason = &validation

		err = errors.New(validation.String())
		return
	}

	if accessToken == nil {
		connKey = key
		return
	}

	claim, err = authService.ClaimJWT(c, *accessToken)
	if err != nil {
		connKey = key
		validation := validation_reason.UNAUTHORIZED
		reason = &validation
		return
	}

	connKey = utils.GetKeyFromIdUserID(msgID, claim.UserID)
	protocol.UpgradeProtocol(key, connKey)

	return
}
