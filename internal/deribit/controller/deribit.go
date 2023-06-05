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
	handler.RegisterHandler("public/get_index_price", handler.getIndexPrice)
	handler.RegisterHandler("public/get_last_trades_by_instrument", handler.getLastTradesByInstrument)

	handler.RegisterHandler("private/buy", handler.buy)
	handler.RegisterHandler("private/sell", handler.sell)
	handler.RegisterHandler("private/edit", handler.edit)
	handler.RegisterHandler("private/cancel", handler.cancel)
	handler.RegisterHandler("private/cancel_all_by_instrument", handler.cancelByInstrument)
	handler.RegisterHandler("private/cancel_all", handler.cancelAll)
	handler.RegisterHandler("private/get_user_trades_by_instrument", handler.getUserTradeByInstrument)
	handler.RegisterHandler("private/get_open_orders_by_instrument", handler.getOpenOrdersByInstrument)
	handler.RegisterHandler("private/get_order_history_by_instrument", handler.getOrderHistoryByInstrument)
	handler.RegisterHandler("private/get_order_state_by_label", handler.getOrderStateByLabel)
	handler.RegisterHandler("private/get_order_state", handler.getOrderState)
	handler.RegisterHandler("private/get_user_trades_by_order", handler.getUserTradesByOrder)

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

func requestHelper(msgID uint64, method string, c *gin.Context) (
	userId,
	connKey string,
	reason *validation_reason.ValidationReason,
	err error,
) {
	key := utils.GetKeyFromIdUserID(msgID, "")
	if isDuplicateConnection := protocol.RegisterProtocolRequest(
		key, protocol.ProtocolRequest{Http: c, Protocol: protocol.HTTP, Method: method},
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

func (h *DeribitHandler) auth(r *gin.Context) {
	type Params struct {
		GrantType    string `json:"grant_type" form:"grant_type"`
		ClientID     string `json:"client_id" form:"client_id"`
		ClientSecret string `json:"client_secret" form:"client_secret"`
		RefreshToken string `json:"refresh_token" form:"refresh_token"`
	}

	var msg deribitModel.RequestDto[Params]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	switch msg.Params.GrantType {
	case "client_credentials":
		payload := userType.AuthRequest{
			APIKey:    msg.Params.ClientID,
			APISecret: msg.Params.ClientSecret,
		}

		if payload.APIKey == "" || payload.APISecret == "" {
			protocol.SendValidationMsg(connKey,
				validation_reason.INVALID_PARAMS, errors.New("required client_id and client_secret"))
			return
		}

		res, _, err := h.authSvc.Login(context.TODO(), payload)
		if err != nil {
			if strings.Contains(err.Error(), "invalid credential") {
				protocol.SendValidationMsg(connKey, validation_reason.UNAUTHORIZED, err)
				return
			}

			protocol.SendErrMsg(connKey, err)
			return
		}

		protocol.SendSuccessMsg(connKey, res)
		return
	case "refresh_token":
		if msg.Params.RefreshToken == "" {
			protocol.SendValidationMsg(connKey,
				validation_reason.INVALID_PARAMS, errors.New("required refresh_token"))
			return
		}

		claim, err := authService.ClaimJWT(nil, msg.Params.RefreshToken)
		if err != nil {
			protocol.SendValidationMsg(connKey, validation_reason.UNAUTHORIZED, err)
			return
		}

		res, _, err := h.authSvc.RefreshToken(context.TODO(), claim)
		if err != nil {
			protocol.SendErrMsg(connKey, err)
			return
		}

		protocol.SendSuccessMsg(connKey, res)
		return
	default:
		protocol.SendValidationMsg(connKey, validation_reason.INVALID_PARAMS, nil)
		return
	}

}

func (h *DeribitHandler) buy(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

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
			protocol.SendValidationMsg(connKey, *validation, err)
			return
		}

		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) sell(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

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
			protocol.SendValidationMsg(connKey, *validation, err)
			return
		}

		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) edit(r *gin.Context) {

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}
	// Call service
	order, err := h.svc.DeribitParseEdit(r.Request.Context(), userID, deribitModel.DeribitEditRequest{
		Id:      msg.Params.Id,
		Price:   msg.Params.Price,
		Amount:  msg.Params.Amount,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) cancel(r *gin.Context) {

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	order, err := h.svc.DeribitParseCancel(r.Request.Context(), userID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.Id,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) cancelByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	order, err := h.svc.DeribitCancelByInstrument(r.Request.Context(), userID, deribitModel.DeribitCancelByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) cancelAll(r *gin.Context) {

	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	order, err := h.svc.DeribitParseCancelAll(r.Request.Context(), userID, deribitModel.DeribitCancelAllRequest{
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	protocol.SendSuccessMsg(connKey, order)
}

func (h *DeribitHandler) getUserTradeByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetUserTradesByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Number of requested items, default - 10
	if msg.Params.Count <= 0 {
		msg.Params.Count = 10
	}

	res := h.svc.DeribitGetUserTradesByInstrument(
		r.Request.Context(),
		userID,
		deribitModel.DeribitGetUserTradesByInstrumentsRequest{
			InstrumentName: msg.Params.InstrumentName,
			Count:          msg.Params.Count,
			StartTimestamp: msg.Params.StartTimestamp,
			EndTimestamp:   msg.Params.EndTimestamp,
			Sorting:        msg.Params.Sorting,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}
func (h *DeribitHandler) getOpenOrdersByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOpenOrdersByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// type parameter
	if msg.Params.Type == "" {
		msg.Params.Type = "all"
	}

	res := h.svc.DeribitGetOpenOrdersByInstrument(
		context.TODO(),
		userId,
		deribitModel.DeribitGetOpenOrdersByInstrumentRequest{
			InstrumentName: msg.Params.InstrumentName,
			Type:           msg.Params.Type,
		},
	)

	protocol.SendSuccessMsg(userId, res)
}

func (h *DeribitHandler) getOrderHistoryByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderHistoryByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// parameter default value
	if msg.Params.Count <= 0 {
		msg.Params.Count = 20
	}

	res := h.svc.DeribitGetOrderHistoryByInstrument(
		context.TODO(),
		userId,
		deribitModel.DeribitGetOrderHistoryByInstrumentRequest{
			InstrumentName:  msg.Params.InstrumentName,
			Count:           msg.Params.Count,
			Offset:          msg.Params.Offset,
			IncludeOld:      msg.Params.IncludeOld,
			IncludeUnfilled: msg.Params.IncludeUnfilled,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}

func (h *DeribitHandler) getInstruments(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetInstrumentsParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := h.svc.DeribitGetInstruments(context.TODO(), deribitModel.DeribitGetInstrumentsRequest{
		Currency: msg.Params.Currency,
		Expired:  msg.Params.Expired,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getOrderBook(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderBookParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := h.svc.GetOrderBook(context.TODO(), deribitModel.DeribitGetOrderBookRequest{
		InstrumentName: msg.Params.InstrumentName,
		Depth:          msg.Params.Depth,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getIndexPrice(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetIndexPriceParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	result := h.svc.GetIndexPrice(context.TODO(), deribitModel.DeribitGetIndexPriceRequest{
		IndexName: msg.Params.IndexName,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getLastTradesByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetLastTradesByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := h.svc.DeribitGetLastTradesByInstrument(context.TODO(), deribitModel.DeribitGetLastTradesByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		StartSeq:       msg.Params.StartSeq,
		EndSeq:         msg.Params.EndSeq,
		StartTimestamp: msg.Params.StartTimestamp,
		EndTimestamp:   msg.Params.EndTimestamp,
		Count:          msg.Params.Count,
		Sorting:        msg.Params.Sorting,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getOrderStateByLabel(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.DeribitGetOrderStateByLabelRequest]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	msg.Params.UserId = userId

	res := h.svc.DeribitGetOrderStateByLabel(r.Request.Context(), msg.Params)

	protocol.SendSuccessMsg(connKey, res)
}

func (h *DeribitHandler) test(r *gin.Context) {
	r.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result": gin.H{
			"version": "1.2.26",
		},
		"testnet": true,
		"usIn":    0,
		"usOut":   0,
		"usDiff":  0,
	})
}

func (h *DeribitHandler) getOrderState(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderStateParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	res := h.svc.DeribitGetOrderState(
		context.TODO(),
		userId,
		deribitModel.DeribitGetOrderStateRequest{
			OrderId: msg.Params.OrderId,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}

func (h *DeribitHandler) getUserTradesByOrder(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetUserTradesByOrderParams]
	if err := utils.UnmarshalAndValidateWS(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	res := h.svc.DeribitGetUserTradesByOrder(
		context.TODO(),
		userId,
		msg.Params.InstrumentName,
		deribitModel.DeribitGetUserTradesByOrderRequest{
			OrderId: msg.Params.OrderId,
			Sorting: msg.Params.Sorting,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}
