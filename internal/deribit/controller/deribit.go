package controller

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/deribit/service"
	"net/http"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/types"

	deribitModel "gateway/internal/deribit/model"
	authService "gateway/internal/user/service"
	userType "gateway/internal/user/types"

	"gateway/pkg/middleware"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"strconv"
	"strings"
	"time"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
	cors "github.com/rs/cors/wrapper/gin"

	"github.com/gin-gonic/gin"
)

type DeribitHandler struct {
	svc     service.IDeribitService
	authSvc authService.IAuthService

	handlers map[string]gin.HandlerFunc
}

func NewDeribitHandler(
	r *gin.Engine,
	svc service.IDeribitService,
	authSvc authService.IAuthService,
) {
	handler := DeribitHandler{
		svc:     svc,
		authSvc: authSvc,
	}

	handler.RegisterHandler("public/auth", handler.auth)
	handler.RegisterHandler("public/get_instruments", handler.getInstruments)
	handler.RegisterHandler("public/test", handler.test)

	handler.RegisterHandler("private/buy", handler.buy)
	handler.RegisterHandler("private/sell", handler.sell)
	handler.RegisterHandler("private/edit", handler.edit)
	handler.RegisterHandler("private/cancel", handler.cancel)
	handler.RegisterHandler("private/cancel_all_by_instrument", handler.cancelByInstrument)
	handler.RegisterHandler("private/cancel_all", handler.cancelAll)
	handler.RegisterHandler("private/get_user_trades_by_instrument", handler.getUserTradeByInstrument)
	handler.RegisterHandler("private/get_open_orders_by_instrument", handler.getOpenOrdersByInstrument)
	handler.RegisterHandler("private/get_order_history_by_instrument", handler.getOrderHistoryByInstrument)

	r.Use(cors.AllowAll())
	r.Use(middleware.Authenticate())

	api := r.Group("/api/v2")
	api.POST("", handler.ApiPostHandler)
	api.GET(":type/*action", handler.ApiGetHandler)
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

func (h *DeribitHandler) auth(r *gin.Context) {
	requestedTime := uint64(time.Now().UnixMicro())

	if ok := protocol.RegisterProtocolRequest(requestedTime, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(requestedTime, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	type Params struct {
		GrantType    string `json:"grant_type" form:"grant_type"`
		ClientID     string `json:"client_id" form:"client_id"`
		ClientSecret string `json:"client_secret" form:"client_secret"`
		RefreshToken string `json:"refresh_token" form:"refresh_token"`
	}

	var msg deribitModel.RequestDto[Params]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(requestedTime, validation_reason.PARSE_ERROR, err)
		return
	}

	switch msg.Params.GrantType {
	case "client_credentials":
		payload := userType.AuthRequest{
			APIKey:    msg.Params.ClientID,
			APISecret: msg.Params.ClientSecret,
		}

		if payload.APIKey == "" || payload.APISecret == "" {
			protocol.SendValidationMsg(requestedTime,
				validation_reason.INVALID_PARAMS, errors.New("required client_id and client_secret"))
			return
		}

		res, _, err := h.authSvc.Login(context.TODO(), payload)
		if err != nil {
			if strings.Contains(err.Error(), "invalid credential") {
				protocol.SendValidationMsg(requestedTime, validation_reason.UNAUTHORIZED, err)
				return
			}

			protocol.SendErrMsg(requestedTime, err)
			return
		}

		protocol.SendSuccessMsg(requestedTime, res)
		return
	case "refresh_token":
		if msg.Params.RefreshToken == "" {
			protocol.SendValidationMsg(requestedTime,
				validation_reason.INVALID_PARAMS, errors.New("required refresh_token"))
			return
		}

		claim, err := authService.ClaimJWT(nil, msg.Params.RefreshToken)
		if err != nil {
			protocol.SendValidationMsg(requestedTime, validation_reason.UNAUTHORIZED, err)
			return
		}

		res, _, err := h.authSvc.RefreshToken(context.TODO(), claim)
		if err != nil {
			protocol.SendErrMsg(requestedTime, err)
			return
		}

		protocol.SendSuccessMsg(requestedTime, res)
		return
	default:
		protocol.SendValidationMsg(requestedTime, validation_reason.INVALID_PARAMS, nil)
		return
	}

}

func (h *DeribitHandler) buy(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, validation, err := h.svc.DeribitRequest(r.Request.Context(), userID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.BUY,
	})
	if err != nil {
		if validation != nil {
			protocol.SendValidationMsg(ID, *validation, err)
			return
		}

		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(ID, order)
}

func (h *DeribitHandler) sell(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, validation, err := h.svc.DeribitRequest(r.Request.Context(), userID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.SELL,
	})
	if err != nil {
		if validation != nil {
			protocol.SendValidationMsg(ID, *validation, err)
			return
		}

		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(ID, order)
}

func (h *DeribitHandler) edit(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, err := h.svc.DeribitParseEdit(r.Request.Context(), userID, deribitModel.DeribitEditRequest{
		Id:      msg.Params.Id,
		Price:   msg.Params.Price,
		Amount:  msg.Params.Amount,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(ID, order)
}

func (h *DeribitHandler) cancel(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, err := h.svc.DeribitParseCancel(r.Request.Context(), userID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.Id,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(ID, order)
}

func (h *DeribitHandler) cancelByInstrument(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, err := h.svc.DeribitCancelByInstrument(r.Request.Context(), userID, deribitModel.DeribitCancelByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(ID, order)
}

func (h *DeribitHandler) cancelAll(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, userID)
	protocol.UpgradeProtocol(userID, ID)

	// Call service
	order, err := h.svc.DeribitParseCancelAll(r.Request.Context(), userID, deribitModel.DeribitCancelAllRequest{
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(ID, err)
		return
	}

	protocol.SendSuccessMsg(ID, order)
}

func (h *DeribitHandler) getUserTradeByInstrument(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	msg := &deribitModel.RequestDto[deribitModel.GetUserTradesByInstrumentParams]{}
	if err := utils.UnmarshalAndValidate(r, msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	// Number of requested items, default - 10
	if msg.Params.Count <= 0 {
		msg.Params.Count = 10
	}

	claim, err := authService.ClaimJWT(nil, msg.Params.AccessToken)
	if err != nil {
		protocol.SendValidationMsg(userID, validation_reason.UNAUTHORIZED, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
	protocol.UpgradeProtocol(userID, ID)

	res := h.svc.DeribitGetUserTradesByInstrument(
		r.Request.Context(),
		claim.UserID,
		deribitModel.DeribitGetUserTradesByInstrumentsRequest{
			InstrumentName: msg.Params.InstrumentName,
			Count:          msg.Params.Count,
			StartTimestamp: msg.Params.StartTimestamp,
			EndTimestamp:   msg.Params.EndTimestamp,
			Sorting:        msg.Params.Sorting,
		},
	)
	protocol.SendSuccessMsg(ID, res)
}
func (h *DeribitHandler) getOpenOrdersByInstrument(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	msg := &deribitModel.RequestDto[deribitModel.GetOpenOrdersByInstrumentParams]{}
	if err := utils.UnmarshalAndValidate(r, msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	// type parameter
	if msg.Params.Type == "" {
		msg.Params.Type = "all"
	}

	claim, err := authService.ClaimJWT(nil, msg.Params.AccessToken)
	if err != nil {
		protocol.SendValidationMsg(userID, validation_reason.UNAUTHORIZED, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
	protocol.UpgradeProtocol(userID, ID)

	res := h.svc.DeribitGetOpenOrdersByInstrument(
		context.TODO(),
		claim.UserID,
		deribitModel.DeribitGetOpenOrdersByInstrumentRequest{
			InstrumentName: msg.Params.InstrumentName,
			Type:           msg.Params.Type,
		},
	)

	protocol.SendSuccessMsg(ID, res)
}

func (h *DeribitHandler) getOrderHistoryByInstrument(r *gin.Context) {
	userID := r.GetString("userID")

	if ok := protocol.RegisterProtocolRequest(userID, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(userID, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	msg := &deribitModel.RequestDto[deribitModel.GetOrderHistoryByInstrumentParams]{}
	if err := utils.UnmarshalAndValidate(r, msg); err != nil {
		protocol.SendValidationMsg(userID, validation_reason.PARSE_ERROR, err)
		return
	}

	// parameter default value
	if msg.Params.Count <= 0 {
		msg.Params.Count = 20
	}

	claim, err := authService.ClaimJWT(nil, msg.Params.AccessToken)
	if err != nil {
		protocol.SendValidationMsg(userID, validation_reason.UNAUTHORIZED, err)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
	protocol.UpgradeProtocol(userID, ID)

	res := h.svc.DeribitGetOrderHistoryByInstrument(
		context.TODO(),
		claim.UserID,
		deribitModel.DeribitGetOrderHistoryByInstrumentRequest{
			InstrumentName:  msg.Params.InstrumentName,
			Count:           msg.Params.Count,
			Offset:          msg.Params.Offset,
			IncludeOld:      msg.Params.IncludeOld,
			IncludeUnfilled: msg.Params.IncludeUnfilled,
		},
	)

	protocol.SendSuccessMsg(ID, res)
}
func (h *DeribitHandler) getInstruments(r *gin.Context) {
	requestedTime := uint64(time.Now().UnixMicro())

	if ok := protocol.RegisterProtocolRequest(requestedTime, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(requestedTime, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	msg := &deribitModel.RequestDto[deribitModel.GetInstrumentsParams]{}
	if err := utils.UnmarshalAndValidate(r, msg); err != nil {
		protocol.SendValidationMsg(requestedTime, validation_reason.PARSE_ERROR, err)
		return
	}

	result := h.svc.DeribitGetInstruments(context.TODO(), deribitModel.DeribitGetInstrumentsRequest{
		Currency: msg.Params.Currency,
		Expired:  msg.Params.Expired,
	})

	protocol.SendSuccessMsg(requestedTime, result)
}

func (h *DeribitHandler) getOrderBook(r *gin.Context) {
	requestedTime := uint64(time.Now().UnixMicro())

	if ok := protocol.RegisterProtocolRequest(requestedTime, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(requestedTime, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	msg := &deribitModel.RequestDto[deribitModel.GetOrderBookParams]{}
	if err := utils.UnmarshalAndValidate(r, msg); err != nil {
		protocol.SendValidationMsg(requestedTime, validation_reason.PARSE_ERROR, err)
		return
	}

	result := h.svc.GetOrderBook(context.TODO(), deribitModel.DeribitGetOrderBookRequest{
		InstrumentName: msg.Params.InstrumentName,
		Depth:          msg.Params.Depth,
	})

	protocol.SendSuccessMsg(requestedTime, result)
}

func (h *DeribitHandler) test(r *gin.Context) {
	requestedTime := uint64(time.Now().UnixMicro())

	if ok := protocol.RegisterProtocolRequest(requestedTime, protocol.HTTP, nil, r); !ok {
		protocol.SendValidationMsg(requestedTime, validation_reason.DUPLICATED_REQUEST_ID, nil)
		return
	}

	protocol.SendSuccessMsg(requestedTime, gin.H{
		"version": "1.2.26",
	})
}
