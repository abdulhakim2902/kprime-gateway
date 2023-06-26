package controller

import (
	"errors"
	"fmt"
	"gateway/internal/deribit/service"
	"net/http"

	"git.devucc.name/dependencies/utilities/commons/logs"

	deribitModel "gateway/internal/deribit/model"
	authService "gateway/internal/user/service"

	"gateway/pkg/memdb"
	"gateway/pkg/middleware"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
	cors "github.com/rs/cors/wrapper/gin"

	"gateway/internal/repositories"

	"github.com/gin-gonic/gin"
)

type DeribitHandler struct {
	svc      service.IDeribitService
	authSvc  authService.IAuthService
	userRepo *repositories.UserRepository
	memDb    *memdb.Schemas

	handlers map[string]gin.HandlerFunc
}

func NewDeribitHandler(
	r *gin.Engine,
	svc service.IDeribitService,
	authSvc authService.IAuthService,
	userRepo *repositories.UserRepository,
	memDb *memdb.Schemas,
) {
	handler := DeribitHandler{
		svc:      svc,
		authSvc:  authSvc,
		userRepo: userRepo,
		memDb:    memDb,
	}

	r.Use(cors.AllowAll())
	r.Use(middleware.Authenticate(memDb))

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
	channelMethods := []string{"private/sell", "private/buy", "private/edit"}
	if utils.ArrContains(channelMethods, method) {
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
