package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	confType "github.com/Undercurrent-Technologies/kprime-utilities/config/types"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"go.mongodb.org/mongo-driver/bson/primitive"

	deribitModel "gateway/internal/deribit/model"
	"gateway/pkg/constant"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"strconv"

	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"

	"github.com/gin-gonic/gin"
)

func (handler *DeribitHandler) RegisterPrivate() {
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
	handler.RegisterHandler("private/get_account_summary", handler.getAccountSummary)

	handler.RegisterHandler("private/get_instruments", handler.getInstruments)
	handler.RegisterHandler("private/get_order_book", handler.getOrderBook)
	handler.RegisterHandler("private/get_tradingview_chart_data", handler.getTradingviewChartData)
}

func (h *DeribitHandler) buy(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	maxShow := 0.1
	if msg.Params.MaxShow == nil {
		msg.Params.MaxShow = &maxShow
	}

	if strings.ToLower(string(msg.Params.Type)) == string(types.LIMIT) && msg.Params.Price == 0 {
		err := errors.New(validation_reason.PRICE_IS_REQUIRED.String())
		sendInvalidRequestMessage(err, msg.Id, validation_reason.PRICE_IS_REQUIRED, r)
		return
	}

	if err := utils.ValidateDeribitRequestParam(msg.Params); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	channel := make(chan protocol.RPCResponseMessage)
	ctx, _ := context.WithTimeout(context.Background(), constant.TIMEOUT)
	go protocol.RegisterChannel(connKey, channel, ctx)

	// Call service
	_, validation, err := h.svc.DeribitRequest(r.Request.Context(), userID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.BUY,
		MaxShow:        *msg.Params.MaxShow,
		ReduceOnly:     msg.Params.ReduceOnly,
		PostOnly:       msg.Params.PostOnly,
	})

	if err != nil {
		if validation != nil {
			sendInvalidRequestMessage(err, msg.Id, *validation, r)
			protocol.UnregisterChannel(connKey)
			return
		}

		sendInvalidRequestMessage(err, msg.Id, validation_reason.PARSE_ERROR, r)
		protocol.UnregisterChannel(connKey)
		return
	}

	res := <-channel
	code := http.StatusOK
	if res.Error != nil {
		code = res.Error.HttpStatusCode
	}
	r.JSON(code, res)
	return
}

func (h *DeribitHandler) sell(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	maxShow := 0.1
	if msg.Params.MaxShow == nil {
		msg.Params.MaxShow = &maxShow
	}

	if strings.ToLower(string(msg.Params.Type)) == string(types.LIMIT) && msg.Params.Price == 0 {
		err := errors.New(validation_reason.PRICE_IS_REQUIRED.String())
		sendInvalidRequestMessage(err, msg.Id, validation_reason.PRICE_IS_REQUIRED, r)
		return
	}

	if err := utils.ValidateDeribitRequestParam(msg.Params); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	channel := make(chan protocol.RPCResponseMessage)
	ctx, _ := context.WithTimeout(context.Background(), constant.TIMEOUT)
	go protocol.RegisterChannel(connKey, channel, ctx)

	// Call service
	_, validation, err := h.svc.DeribitRequest(r.Request.Context(), userID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.SELL,
		MaxShow:        *msg.Params.MaxShow,
		ReduceOnly:     msg.Params.ReduceOnly,
		PostOnly:       msg.Params.PostOnly,
	})
	if err != nil {
		if validation != nil {
			sendInvalidRequestMessage(err, msg.Id, *validation, r)
			protocol.UnregisterChannel(connKey)
			return
		}

		sendInvalidRequestMessage(err, msg.Id, validation_reason.PARSE_ERROR, r)
		protocol.UnregisterChannel(connKey)
		return
	}
	res := <-channel
	code := http.StatusOK
	if res.Error != nil {
		code = res.Error.HttpStatusCode
	}
	r.JSON(code, res)
}

func (h *DeribitHandler) edit(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.EditParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	// Validate order id, make sure it's a valid order id (Mongodb object id)
	_, err := primitive.ObjectIDFromHex(msg.Params.OrderId)
	if err != nil {
		errMsg := protocol.ErrorMessage{
			Message:        constant.INVALID_ORDER_ID,
			Data:           protocol.ReasonMessage{},
			HttpStatusCode: http.StatusBadRequest,
		}
		m := protocol.RPCResponseMessage{
			JSONRPC: "2.0",
			ID:      msg.Id,
			Error:   &errMsg,
			Testnet: true,
		}
		r.AbortWithStatusJSON(http.StatusBadRequest, m)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}
	channel := make(chan protocol.RPCResponseMessage)
	ctx, _ := context.WithTimeout(context.Background(), constant.TIMEOUT)
	go protocol.RegisterChannel(connKey, channel, ctx)
	// Call service
	_, reason, err = h.svc.DeribitParseEdit(r.Request.Context(), userID, deribitModel.DeribitEditRequest{
		Id:      msg.Params.OrderId,
		Price:   msg.Params.Price,
		Amount:  msg.Params.Amount,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		if reason != nil {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
			protocol.UnregisterChannel(connKey)
			return
		}

		sendInvalidRequestMessage(err, msg.Id, validation_reason.PARSE_ERROR, r)
		protocol.UnregisterChannel(connKey)
		return
	}

	res := <-channel
	code := http.StatusOK
	if res.Error != nil {
		code = res.Error.HttpStatusCode
	}
	r.JSON(code, res)
}

func (h *DeribitHandler) cancel(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.CancelParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	// Validate order id, make sure it's a valid order id (Mongodb object id)
	_, err := primitive.ObjectIDFromHex(msg.Params.OrderId)
	if err != nil {
		errMsg := protocol.ErrorMessage{
			Message:        constant.INVALID_ORDER_ID,
			Data:           protocol.ReasonMessage{},
			HttpStatusCode: http.StatusBadRequest,
		}
		m := protocol.RPCResponseMessage{
			JSONRPC: "2.0",
			ID:      msg.Id,
			Error:   &errMsg,
			Testnet: true,
		}
		r.AbortWithStatusJSON(http.StatusBadRequest, m)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}
	channel := make(chan protocol.RPCResponseMessage)
	ctx, _ := context.WithTimeout(context.Background(), constant.TIMEOUT)
	go protocol.RegisterChannel(connKey, channel, ctx)
	// Call service
	_, err = h.svc.DeribitParseCancel(r.Request.Context(), userID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.OrderId,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.PARSE_ERROR, r)
		protocol.UnregisterChannel(connKey)
		return
	}

	res := <-channel
	code := http.StatusOK
	if res.Error != nil {
		code = res.Error.HttpStatusCode
	}
	r.JSON(code, res)
}

func (h *DeribitHandler) cancelByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.CancelByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	_, err := utils.ParseInstruments(msg.Params.InstrumentName, false)
	if err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	channel := make(chan protocol.RPCResponseMessage)
	ctx, _ := context.WithTimeout(context.Background(), constant.TIMEOUT)
	go protocol.RegisterChannel(connKey, channel, ctx)

	// Call service
	_, err = h.svc.DeribitCancelByInstrument(r.Request.Context(), userID, deribitModel.DeribitCancelByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		Type:           types.Type(msg.Params.Type),
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		protocol.UnregisterChannel(connKey)
		return
	}

	res := <-channel
	code := http.StatusOK
	if res.Error != nil {
		code = res.Error.HttpStatusCode
	}
	r.JSON(code, res)
}

func (h *DeribitHandler) cancelAll(r *gin.Context) {

	var msg deribitModel.RequestDto[deribitModel.CancelOnDisconnectParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	channel := make(chan protocol.RPCResponseMessage)
	ctx, _ := context.WithTimeout(context.Background(), constant.TIMEOUT)
	go protocol.RegisterChannel(connKey, channel, ctx)

	// Call service
	_, err = h.svc.DeribitParseCancelAll(r.Request.Context(), userID, deribitModel.DeribitCancelAllRequest{
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		protocol.UnregisterChannel(connKey)
		return
	}

	res := <-channel
	code := http.StatusOK
	if res.Error != nil {
		code = res.Error.HttpStatusCode
	}
	r.JSON(code, res)
}

func (h *DeribitHandler) getUserTradeByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetUserTradesByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	// Number of requested items, default - 10
	if msg.Params.Count <= 0 {
		msg.Params.Count = 10
	}

	_, err = utils.ParseInstruments(msg.Params.InstrumentName, false)
	if err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
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
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	_, err = utils.ParseInstruments(msg.Params.InstrumentName, false)
	if err != nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, err)
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

	protocol.SendSuccessMsg(connKey, res)
}

func (h *DeribitHandler) getOrderHistoryByInstrument(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderHistoryByInstrumentParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	// parameter default value
	if msg.Params.Count <= 0 {
		msg.Params.Count = 20
	}

	_, err = utils.ParseInstruments(msg.Params.InstrumentName, false)
	if err != nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, err)
		return
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

func (h *DeribitHandler) getOrderStateByLabel(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.DeribitGetOrderStateByLabelRequest]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		reason := validation_reason.PARSE_ERROR
		code, _, codeStr := reason.Code()

		m := protocol.RPCResponseMessage{
			JSONRPC: "2.0",
			ID:      msg.Id,
			Error: &protocol.ErrorMessage{
				Message: err.Error(),
				Data: protocol.ReasonMessage{
					Reason: codeStr,
				},
				Code: code,
			},
			Testnet: true,
		}
		r.AbortWithStatusJSON(http.StatusBadRequest, m)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	currency, ok := confType.Pair(msg.Params.Currency).CurrencyCheck()
	if !ok {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid currency"))
		return
	}

	msg.Params.UserId = userId
	msg.Params.Currency = currency

	res := h.svc.DeribitGetOrderStateByLabel(r.Request.Context(), msg.Params)

	protocol.SendSuccessMsg(connKey, res)
}

func (h *DeribitHandler) getOrderState(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderStateParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		reason := validation_reason.PARSE_ERROR
		code, _, codeStr := reason.Code()

		m := protocol.RPCResponseMessage{
			JSONRPC: "2.0",
			ID:      msg.Id,
			Error: &protocol.ErrorMessage{
				Message: err.Error(),
				Data: protocol.ReasonMessage{
					Reason: codeStr,
				},
				Code: code,
			},
			Testnet: true,
		}
		r.AbortWithStatusJSON(http.StatusBadRequest, m)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	_, err = primitive.ObjectIDFromHex(msg.Params.OrderId)
	if err != nil {
		reason := validation_reason.INVALID_PARAMS
		protocol.SendValidationMsg(connKey, reason, errors.New("invalid order_id"))
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

	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		reason := validation_reason.PARSE_ERROR
		code, _, codeStr := reason.Code()

		m := protocol.RPCResponseMessage{
			JSONRPC: "2.0",
			ID:      msg.Id,
			Error: &protocol.ErrorMessage{
				Message: err.Error(),
				Data: protocol.ReasonMessage{
					Reason: codeStr,
				},
				Code: code,
			},
			Testnet: true,
		}
		r.AbortWithStatusJSON(http.StatusBadRequest, m)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	_, err = primitive.ObjectIDFromHex(msg.Params.OrderId)
	if err != nil {
		reason := validation_reason.INVALID_PARAMS
		protocol.SendValidationMsg(connKey, reason, errors.New("invalid order_id"))
		return
	}

	res := h.svc.DeribitGetUserTradesByOrder(
		context.TODO(),
		userId,
		deribitModel.DeribitGetUserTradesByOrderRequest{
			OrderId: msg.Params.OrderId,
			Sorting: msg.Params.Sorting,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}

func (h *DeribitHandler) getAccountSummary(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetAccountSummary]

	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	result := h.svc.FetchUserBalance(
		msg.Params.Currency,
		userId,
	)
	balance, _ := strconv.ParseFloat(result.Balance, 64)

	user, _ := h.userRepo.FindById(context.TODO(), userId)
	resp := deribitModel.GetAccountSummaryResponse{
		Id:                userId,
		Currency:          msg.Params.Currency,
		Email:             user.Email,
		Balance:           balance,
		MarginBalance:     balance,
		CreationTimestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}

	protocol.SendSuccessMsg(connKey, resp)
}

func (h *DeribitHandler) getInstruments(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetInstrumentsParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		sendInvalidRequestMessage(err, msg.Id, validation_reason.INVALID_PARAMS, r)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	currency, ok := confType.Pair(msg.Params.Currency).CurrencyCheck()
	if !ok {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid currency"))
		return
	}

	if msg.Params.Kind != "" && strings.ToLower(msg.Params.Kind) != "option" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid value of kind"))
		return
	}

	if msg.Params.IncludeSpots {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid value of include_spots"))
		return
	}

	result := h.svc.DeribitGetInstruments(context.TODO(), deribitModel.DeribitGetInstrumentsRequest{
		Currency: currency,
		Expired:  msg.Params.Expired,
		UserId:   userId,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getOrderBook(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderBookParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		reason := validation_reason.PARSE_ERROR
		code, _, codeStr := reason.Code()

		errMsg := protocol.ErrorMessage{
			Message: err.Error(),
			Data: protocol.ReasonMessage{
				Reason: codeStr,
			},
			Code: code,
		}
		m := protocol.RPCResponseMessage{
			JSONRPC: "2.0",
			ID:      msg.Id,
			Error:   &errMsg,
			Testnet: true,
		}
		r.AbortWithStatusJSON(http.StatusBadRequest, m)
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	instruments, _ := utils.ParseInstruments(msg.Params.InstrumentName, false)

	if instruments == nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("instrument not found"))
		return
	}

	result := h.svc.GetOrderBook(context.TODO(), deribitModel.DeribitGetOrderBookRequest{
		InstrumentName: msg.Params.InstrumentName,
		Depth:          msg.Params.Depth,
		UserId:         userId,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (h *DeribitHandler) getTradingviewChartData(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.GetTradingviewChartDataRequest]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		r.AbortWithStatusJSON(http.StatusBadRequest, map[string]map[string]any{"error": {"message": err.Error()}})
		return
	}

	userId, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			sendInvalidRequestMessage(err, msg.Id, *reason, r)
		}
		return
	}

	_, err = utils.ParseInstruments(msg.Params.InstrumentName, false)
	if err != nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, err)
		return
	}
	msg.Params.UserId = userId

	result, reason, err := h.svc.GetTradingViewChartData(context.TODO(), msg.Params)
	if err != nil {
		if reason != nil {
			protocol.SendValidationMsg(connKey, *reason, err)
			return
		}

		protocol.SendErrMsg(connKey, err)
		return

	}

	protocol.SendSuccessMsg(connKey, result)
}
