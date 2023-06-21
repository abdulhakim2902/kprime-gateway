package controller

import (
	"context"
	"errors"
	"fmt"
	deribitModel "gateway/internal/deribit/model"
	"gateway/pkg/middleware"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"gateway/pkg/ws"
	"strconv"
	"strings"
	"time"

	"git.devucc.name/dependencies/utilities/types"
	"git.devucc.name/dependencies/utilities/types/validation_reason"
)

func (handler *wsHandler) RegisterPrivate() {
	ws.RegisterChannel("private/buy", middleware.MiddlewaresWrapper(handler.PrivateBuy, middleware.RateLimiterWs))
	ws.RegisterChannel("private/sell", middleware.MiddlewaresWrapper(handler.PrivateSell, middleware.RateLimiterWs))
	ws.RegisterChannel("private/edit", middleware.MiddlewaresWrapper(handler.PrivateEdit, middleware.RateLimiterWs))
	ws.RegisterChannel("private/cancel", middleware.MiddlewaresWrapper(handler.PrivateCancel, middleware.RateLimiterWs))
	ws.RegisterChannel("private/cancel_all_by_instrument", middleware.MiddlewaresWrapper(handler.PrivateCancelByInstrument, middleware.RateLimiterWs))
	ws.RegisterChannel("private/cancel_all", middleware.MiddlewaresWrapper(handler.PrivateCancelAll, middleware.RateLimiterWs))
	ws.RegisterChannel("private/get_user_trades_by_order", middleware.MiddlewaresWrapper(handler.PrivateGetUserTradesByOrder, middleware.RateLimiterWs))
	ws.RegisterChannel("private/get_user_trades_by_instrument", middleware.MiddlewaresWrapper(handler.PrivateGetUserTradesByInstrument, middleware.RateLimiterWs))
	ws.RegisterChannel("private/get_open_orders_by_instrument", middleware.MiddlewaresWrapper(handler.PrivateGetOpenOrdersByInstrument, middleware.RateLimiterWs))
	ws.RegisterChannel("private/get_order_history_by_instrument", middleware.MiddlewaresWrapper(handler.PrivateGetOrderHistoryByInstrument, middleware.RateLimiterWs))
	ws.RegisterChannel("private/get_order_state_by_label", middleware.MiddlewaresWrapper(handler.PrivateGetOrderStateByLabel, middleware.RateLimiterWs))
	ws.RegisterChannel("private/get_order_state", middleware.MiddlewaresWrapper(handler.PrivateGetOrderState, middleware.RateLimiterWs))
	ws.RegisterChannel("private/get_account_summary", middleware.MiddlewaresWrapper(handler.PrivateGetAccountSummary, middleware.RateLimiterWs))
	ws.RegisterChannel("private/subscribe", middleware.MiddlewaresWrapper(handler.PrivateSubscribeHandler, middleware.RateLimiterWs))
	ws.RegisterChannel("private/unsubscribe", middleware.MiddlewaresWrapper(handler.PrivateUnsubscribeHandler, middleware.RateLimiterWs))
	ws.RegisterChannel("private/unsubscribe_all", middleware.MiddlewaresWrapper(handler.PrivateUnsubscribeAllHandler, middleware.RateLimiterWs))
}

func (svc wsHandler) PrivateBuy(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	maxShow := 0.1
	if msg.Params.MaxShow == nil {
		msg.Params.MaxShow = &maxShow
	}

	if err := utils.ValidateDeribitRequestParam(msg.Params); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Parse the Deribit BUY
	_, validation, err := svc.deribitSvc.DeribitRequest(context.TODO(), claim.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.BUY,
		MaxShow:        *msg.Params.MaxShow,
		PostOnly:       msg.Params.PostOnly,
		ReduceOnly:     msg.Params.ReduceOnly,
	})
	if err != nil {
		if validation != nil {
			protocol.SendValidationMsg(connKey, *validation, err)
			return
		}

		protocol.SendErrMsg(connKey, err)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(connKey, c)
}

func (svc wsHandler) PrivateSell(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	maxShow := 0.1
	if msg.Params.MaxShow == nil {
		msg.Params.MaxShow = &maxShow
	}

	if err := utils.ValidateDeribitRequestParam(msg.Params); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Parse the Deribit Sell
	_, validation, err := svc.deribitSvc.DeribitRequest(context.TODO(), claim.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.SELL,
		MaxShow:        *msg.Params.MaxShow,
		PostOnly:       msg.Params.PostOnly,
		ReduceOnly:     msg.Params.ReduceOnly,
	})
	if err != nil {
		if validation != nil {
			protocol.SendValidationMsg(connKey, *validation, err)
			return
		}

		protocol.SendErrMsg(connKey, err)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(connKey, c)
}

func (svc wsHandler) PrivateEdit(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseEdit(context.TODO(), claim.UserID, deribitModel.DeribitEditRequest{
		Id:      msg.Params.Id,
		Price:   msg.Params.Price,
		Amount:  msg.Params.Amount,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(connKey, c)
}

func (svc wsHandler) PrivateCancel(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseCancel(context.TODO(), claim.UserID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.Id,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(connKey, c)
}

func (svc wsHandler) PrivateCancelByInstrument(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, nil)
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitCancelByInstrument(context.TODO(), claim.UserID, deribitModel.DeribitCancelByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	//register order connection
	ws.RegisterOrderConnection(connKey, c)
}

func (svc wsHandler) PrivateCancelAll(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.RequestParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseCancelAll(context.TODO(), claim.UserID, deribitModel.DeribitCancelAllRequest{
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		protocol.SendErrMsg(connKey, err)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(connKey, c)
}

func (svc wsHandler) PrivateGetUserTradesByInstrument(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetUserTradesByInstrumentParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// Number of requested items, default - 10
	if msg.Params.Count <= 0 {
		msg.Params.Count = 10
	}

	res := svc.wsTradeSvc.GetUserTradesByInstrument(
		context.TODO(),
		claim.UserID,
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

func (svc wsHandler) PrivateGetOpenOrdersByInstrument(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetOpenOrdersByInstrumentParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// type parameter
	if msg.Params.Type == "" {
		msg.Params.Type = "all"
	}

	res := svc.wsOSvc.GetOpenOrdersByInstrument(
		context.TODO(),
		claim.UserID,
		deribitModel.DeribitGetOpenOrdersByInstrumentRequest{
			InstrumentName: msg.Params.InstrumentName,
			Type:           msg.Params.Type,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}

func (svc wsHandler) PrivateGetOrderHistoryByInstrument(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderHistoryByInstrumentParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// parameter default value
	if msg.Params.Count <= 0 {
		msg.Params.Count = 20
	}

	res := svc.wsOSvc.GetGetOrderHistoryByInstrument(
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

	protocol.SendSuccessMsg(connKey, res)
}

func (svc wsHandler) PrivateGetUserTradesByOrder(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetUserTradesByOrderParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	res := svc.wsTradeSvc.GetUserTradesByOrder(
		context.TODO(),
		claim.UserID,
		deribitModel.DeribitGetUserTradesByOrderRequest{
			OrderId: msg.Params.OrderId,
			Sorting: msg.Params.Sorting,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}

func (svc wsHandler) PrivateGetOrderState(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderStateParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	res := svc.wsOSvc.GetOrderState(
		context.TODO(),
		claim.UserID,
		deribitModel.DeribitGetOrderStateRequest{
			OrderId: msg.Params.OrderId,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}

func (svc wsHandler) PrivateGetAccountSummary(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetAccountSummary]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := svc.wsUserBalanceSvc.FetchUserBalance(
		msg.Params.Currency,
		claim.UserID,
	)
	balance, _ := strconv.ParseFloat(result.Balance, 64)

	user, _ := svc.userRepo.FindById(context.TODO(), claim.UserID)
	resp := deribitModel.GetAccountSummaryResponse{
		Id:                claim.UserID,
		Currency:          msg.Params.Currency,
		Email:             user.Email,
		Balance:           balance,
		MarginBalance:     balance,
		CreationTimestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}

	protocol.SendSuccessMsg(connKey, resp)
}

func (svc wsHandler) PrivateGetOrderStateByLabel(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.DeribitGetOrderStateByLabelRequest]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	msg.Params.UserId = claim.UserID

	res := svc.deribitSvc.DeribitGetOrderStateByLabel(context.TODO(), msg.Params)

	protocol.SendSuccessMsg(connKey, res)
}

func (svc wsHandler) PrivateUnsubscribeAllHandler(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.ChannelParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	svc.wsOSvc.Unsubscribe(c)
	svc.wsTradeSvc.Unsubscribe(c)
	svc.wsOBSvc.Unsubscribe(c)

	protocol.SendSuccessMsg(connKey, "ok")
}

func (svc wsHandler) PrivateSubscribeHandler(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.ChannelParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	claim, connKey, reason, err := requestHelper(msg.Id, msg.Method, &msg.Params.AccessToken, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	const t = true
	method := map[string]bool{"orders": t, "trades": t, "changes": t}
	interval := map[string]bool{"raw": t, "100ms": t, "agg2": t}
	for _, channel := range msg.Params.Channels {
		s := strings.Split(channel, ".")
		if len(s) == 0 {
			err := errors.New("error invalid channel")
			c.SendInvalidRequestMessage(err)
			return
		}
		val, ok := method[s[1]]
		if !ok {
			err := errors.New("error invalid channel")
			c.SendInvalidRequestMessage(err)
			return
		}

		if val {
			if len(s) < 4 {
				err := errors.New("error invalid interval")
				c.SendInvalidRequestMessage(err)
				return
			}
			if _, ok := interval[s[3]]; !ok {
				err := errors.New("error invalid interval")
				c.SendInvalidRequestMessage(err)
				return
			}
		}
	}

	protocol.SendSuccessMsg(connKey, msg.Params.Channels)

	for _, channel := range msg.Params.Channels {
		s := strings.Split(channel, ".")
		if len(s) != 4 {
			reason := validation_reason.INVALID_PARAMS
			err := fmt.Errorf("unrecognize channel for '%s'", channel)
			protocol.SendValidationMsg(connKey, reason, err)
			return
		}

		switch s[1] {
		case "orders":
			svc.wsOSvc.SubscribeUserOrder(c, channel, claim.UserID)
		case "trades":
			svc.wsTradeSvc.SubscribeUserTrades(c, channel, claim.UserID)
		case "changes":
			svc.wsOBSvc.SubscribeUserChange(c, channel, claim.UserID)
		default:
			reason := validation_reason.INVALID_PARAMS
			err := fmt.Errorf("unrecognize channel for '%s'", channel)
			protocol.SendValidationMsg(connKey, reason, err)
			return
		}
	}

	protocol.SendSuccessMsg(connKey, msg.Params.Channels)

}

func (svc wsHandler) PrivateUnsubscribeHandler(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.ChannelParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	protocol.SendSuccessMsg(connKey, msg.Params.Channels)

	for _, channel := range msg.Params.Channels {
		s := strings.Split(channel, ".")
		switch s[1] {
		case "orders":
			svc.wsOSvc.Unsubscribe(c)
		case "trades":
			svc.wsTradeSvc.Unsubscribe(c)
		case "changes":
			svc.wsOBSvc.Unsubscribe(c)
		}

	}

}
