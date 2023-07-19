package controller

import (
	"errors"
	"fmt"
	"gateway/internal/deribit/service"
	"net/http"
	"strings"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"

	deribitModel "gateway/internal/deribit/model"
	authService "gateway/internal/user/service"

	"gateway/pkg/middleware"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"

	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"
	cors "github.com/rs/cors/wrapper/gin"

	"gateway/internal/repositories"

	"github.com/gin-gonic/gin"
)

type DeribitHandler struct {
	svc      service.IDeribitService
	authSvc  authService.IAuthService
	userRepo *repositories.UserRepository

	handlers map[string]gin.HandlerFunc
}

func NewDeribitHandler(
	r *gin.Engine,
	svc service.IDeribitService,
	authSvc authService.IAuthService,
	userRepo *repositories.UserRepository,
) {
	handler := DeribitHandler{
		svc:      svc,
		authSvc:  authSvc,
		userRepo: userRepo,
	}

	r.Use(cors.AllowAll())
	r.Use(middleware.Authenticate())

	api := r.Group("/api/v2")
	api.POST("", handler.ApiPostHandler)
	api.GET(":type/*action", handler.ApiGetHandler)

	handler.RegisterPrivate()
	handler.RegisterPublic()
}

func (h *DeribitHandler) RegisterHandler(method string, handler gin.HandlerFunc) {
	if h.handlers == nil {
		h.handlers = make(map[string]gin.HandlerFunc)
	}

	h.handlers[method] = handler
}

func (h *DeribitHandler) ApiPostHandler(r *gin.Context) {
	type Params struct{}

	var dto deribitModel.RequestDto[Params]
	if err := utils.UnmarshalAndValidate(r, &dto); err != nil {
		r.AbortWithStatusJSON(http.StatusBadRequest, err.Error())
		return
	}

	handler, ok := h.handlers[dto.Method]
	if !ok {
		r.AbortWithStatus(http.StatusNotFound)
		return
	}

	logs.Log.Info().Str("called_method", dto.Method).Msg("")

	handler(r)
}

func (h *DeribitHandler) ApiGetHandler(r *gin.Context) {
	method := fmt.Sprintf("%s%s", r.Param("type"), r.Param("action"))

	handler, ok := h.handlers[method]
	if !ok {
		r.AbortWithStatus(http.StatusNotFound)
		return
	}

	logs.Log.Info().Str("called_method", method).Msg("")

	handler(r)
}

func requestHelper(msgID uint64, method string, c *gin.Context) (
	userId,
	connKey string,
	reason *validation_reason.ValidationReason,
	err error,
) {
	prtcl := protocol.HTTP
	channelMethods := []string{"private/sell", "private/buy", "private/edit", "private/cancel", "private/cancel_all_by_instrument", "private/cancel_all"}

	// Defining method for get requests
	url := c.Request.URL.Path
	strs := strings.Split(url, "/")
	getMethod := strs[len(strs)-2] + "/" + strs[len(strs)-1]

	if utils.ArrContains(channelMethods, method) || utils.ArrContains(channelMethods, getMethod) {
		prtcl = protocol.Channel
	}

	key := utils.GetKeyFromIdUserID(msgID, "")
	if isDuplicateConnection := protocol.RegisterProtocolRequest(
		key, protocol.ProtocolRequest{Http: c, Protocol: prtcl, Method: method},
	); isDuplicateConnection {
		validation := validation_reason.DUPLICATED_REQUEST_ID
		reason = &validation

		err = errors.New(validation.String())
		return
	}

	userId = c.GetString("userID")

	if len(userId) == 0 {
		connKey = key
		return
	}

	connKey = utils.GetKeyFromIdUserID(msgID, userId)
	protocol.UpgradeProtocol(key, connKey)

	return
}

func sendInvalidRequestMessage(err error, msgId uint64, reason validation_reason.ValidationReason, r *gin.Context) {
	reasonMsg := protocol.ReasonMessage{
		Reason: reason.String(),
	}
	_, httpCode, _ := reason.Code()

	errMsg := protocol.ErrorMessage{
		Message:        err.Error(),
		Data:           reasonMsg,
		HttpStatusCode: httpCode,
	}
	m := protocol.RPCResponseMessage{
		JSONRPC: "2.0",
		ID:      msgId,
		Error:   &errMsg,
		Testnet: true,
	}
	r.AbortWithStatusJSON(http.StatusBadRequest, m)
}
