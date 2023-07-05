package controller

import (
	"context"
	"net/http"
	"time"

	"git.devucc.name/dependencies/utilities/types"

	deribitModel "gateway/internal/deribit/model"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"strconv"

	"git.devucc.name/dependencies/utilities/types/validation_reason"

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
}

func (h *DeribitHandler) buy(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		errMsg := protocol.ErrorMessage{
			Message:        err.Error(),
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

	maxShow := 0.1
	if msg.Params.MaxShow == nil {
		msg.Params.MaxShow = &maxShow
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	channel := make(chan protocol.RPCResponseMessage)
	go protocol.RegisterChannel(connKey, channel)
	if err := utils.ValidateDeribitRequestParam(msg.Params); err != nil {
		protocol.SendValidationMsg(connKey, validation_reason.INVALID_PARAMS, err)
	}

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
			protocol.SendValidationMsg(connKey, *validation, err)
		}

		protocol.SendErrMsg(connKey, err)
	}

	res := <-channel
	code := http.StatusOK
	if res.Error != nil {
		code = res.Error.HttpStatusCode
	}
	r.JSON(code, res)
}

func (h *DeribitHandler) sell(r *gin.Context) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
		errMsg := protocol.ErrorMessage{
			Message:        err.Error(),
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

	maxShow := 0.1
	if msg.Params.MaxShow == nil {
		msg.Params.MaxShow = &maxShow
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	channel := make(chan protocol.RPCResponseMessage)
	go protocol.RegisterChannel(connKey, channel)
	if err := utils.ValidateDeribitRequestParam(msg.Params); err != nil {
		protocol.SendValidationMsg(connKey, validation_reason.INVALID_PARAMS, err)
	}

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
			protocol.SendValidationMsg(connKey, *validation, err)
		}
		protocol.SendErrMsg(connKey, err)
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
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}
	// Call service
	_, err = h.svc.DeribitParseEdit(r.Request.Context(), userID, deribitModel.DeribitEditRequest{
		Id:      msg.Params.OrderId,
		Price:   msg.Params.Price,
		Amount:  msg.Params.Amount,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}
	channel := make(chan protocol.RPCResponseMessage)
	go protocol.RegisterChannel(connKey, channel)
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
		errMsg := protocol.ErrorMessage{
			Message:        err.Error(),
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
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	_, err = h.svc.DeribitParseCancel(r.Request.Context(), userID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.OrderId,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	channel := make(chan protocol.RPCResponseMessage)
	go protocol.RegisterChannel(connKey, channel)
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
		r.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, connKey, reason, err := requestHelper(msg.Id, msg.Method, r)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Call service
	_, err = h.svc.DeribitCancelByInstrument(r.Request.Context(), userID, deribitModel.DeribitCancelByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	channel := make(chan protocol.RPCResponseMessage)
	go protocol.RegisterChannel(connKey, channel)
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

	if err := utils.UnmarshalAndValidate(r, &msg); err != nil {
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
		protocol.SendValidationMsg(connKey, *reason, err)
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
