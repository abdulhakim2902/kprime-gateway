package controller

import (
	"context"
	"errors"
	"fmt"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"strconv"
	"strings"

	deribitModel "gateway/internal/deribit/model"
	"gateway/internal/repositories"
	userType "gateway/internal/user/types"

	deribitService "gateway/internal/deribit/service"
	authService "gateway/internal/user/service"
	engService "gateway/internal/ws/engine/service"
	wsService "gateway/internal/ws/service"

	"git.devucc.name/dependencies/utilities/types"
	"git.devucc.name/dependencies/utilities/types/validation_reason"
	cors "github.com/rs/cors/wrapper/gin"

	"gateway/pkg/ws"

	"github.com/gin-gonic/gin"
)

type wsHandler struct {
	authSvc       authService.IAuthService
	deribitSvc    deribitService.IDeribitService
	wsOBSvc       wsService.IwsOrderbookService
	wsOSvc        wsService.IwsOrderService
	wsEngSvc      engService.IwsEngineService
	wsTradeSvc    wsService.IwsTradeService
	wsRawPriceSvc wsService.IwsRawPriceService

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
	userRepo *repositories.UserRepository,
) {
	handler := &wsHandler{
		authSvc:       authSvc,
		deribitSvc:    deribitSvc,
		wsOBSvc:       wsOBSvc,
		wsEngSvc:      wsEngSvc,
		wsOSvc:        wsOSvc,
		wsTradeSvc:    wsTradeSvc,
		wsRawPriceSvc: wsRawPriceSvc,
		userRepo:      userRepo,
	}
	r.Use(cors.AllowAll())

	r.GET("/ws/api/v2", ws.ConnectionEndpoint)

	ws.RegisterChannel("public/auth", handler.PublicAuth)
	ws.RegisterChannel("private/buy", handler.PrivateBuy)
	ws.RegisterChannel("private/sell", handler.PrivateSell)
	ws.RegisterChannel("private/edit", handler.PrivateEdit)
	ws.RegisterChannel("private/cancel", handler.PrivateCancel)
	ws.RegisterChannel("private/cancel_all_by_instrument", handler.PrivateCancelByInstrument)
	ws.RegisterChannel("private/cancel_all", handler.PrivateCancelAll)
	ws.RegisterChannel("private/get_user_trades_by_order", handler.PrivateGetUserTradesByOrder)
	ws.RegisterChannel("private/get_user_trades_by_instrument", handler.PrivateGetUserTradesByInstrument)
	ws.RegisterChannel("private/get_open_orders_by_instrument", handler.PrivateGetOpenOrdersByInstrument)
	ws.RegisterChannel("private/get_order_history_by_instrument", handler.PrivateGetOrderHistoryByInstrument)
	ws.RegisterChannel("private/get_order_state_by_label", handler.PrivateGetOrderStateByLabel)
	ws.RegisterChannel("private/get_order_state", handler.PrivateGetOrderState)

	ws.RegisterChannel("public/subscribe", handler.SubscribeHandler)
	ws.RegisterChannel("public/unsubscribe", handler.UnsubscribeHandler)
	ws.RegisterChannel("public/unsubscribe_all", handler.UnsubscribeAllHandler)

	ws.RegisterChannel("private/subscribe", handler.SubscribeHandlerPrivate)
	ws.RegisterChannel("private/unsubscribe", handler.UnsubscribeHandlerPrivate)
	ws.RegisterChannel("private/unsubscribe_all", handler.UnsubscribeAllHandlerPrivate)

	ws.RegisterChannel("public/get_instruments", handler.GetInstruments)
	ws.RegisterChannel("public/get_last_trades_by_instrument", handler.GetLastTradesByInstrument)

	ws.RegisterChannel("public/get_order_book", handler.GetOrderBook)
	ws.RegisterChannel("public/get_index_price", handler.GetIndexPrice)
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

func (svc wsHandler) PublicAuth(input interface{}, c *ws.Client) {
	type Params struct {
		GrantType    string `json:"grant_type" validate:"required"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`
	}

	var msg deribitModel.RequestDto[Params]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	var res any
	var user *userType.User

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

		res, user, err = svc.authSvc.Login(context.TODO(), payload)
		if err != nil {
			if strings.Contains(err.Error(), "invalid credential") {
				protocol.SendValidationMsg(connKey, validation_reason.UNAUTHORIZED, err)
				return
			}

			protocol.SendErrMsg(connKey, err)
			return
		}
	case "refresh_token":
		if msg.Params.RefreshToken == "" {
			protocol.SendValidationMsg(connKey,
				validation_reason.INVALID_PARAMS, errors.New("required refresh_token"))
			return
		}

		claim, err := authService.ClaimJWT(c, msg.Params.RefreshToken)
		if err != nil {
			protocol.SendValidationMsg(connKey, validation_reason.UNAUTHORIZED, err)
			return
		}

		res, user, err = svc.authSvc.RefreshToken(context.TODO(), claim)
		if err != nil {
			protocol.SendErrMsg(connKey, err)
			return
		}
	}

	c.RegisterAuthedConnection(user.ID.Hex())

	protocol.SendSuccessMsg(connKey, res)
}

func (svc wsHandler) PrivateBuy(input interface{}, c *ws.Client) {
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

func (svc wsHandler) SubscribeHandler(input interface{}, c *ws.Client) {
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
		fmt.Println(channel)
		s := strings.Split(channel, ".")
		switch s[0] {
		case "orderbook":
			svc.wsOBSvc.Subscribe(c, s[1])
		case "engine":
			svc.wsEngSvc.Subscribe(c, s[1])
		case "order":
			svc.wsOSvc.Subscribe(c, s[1])
		case "trade":
			svc.wsTradeSvc.Subscribe(c, s[1])
		case "trades":
			svc.wsTradeSvc.SubscribeTrades(c, channel)
		case "quote":
			svc.wsOBSvc.SubscribeQuote(c, s[1])
		case "book":
			svc.wsOBSvc.SubscribeBook(c, channel)
		case "deribit_price_index":
			svc.wsRawPriceSvc.Subscribe(c, s[1])
		}
	}
}

func (svc wsHandler) UnsubscribeHandler(input interface{}, c *ws.Client) {
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
		switch s[0] {
		case "orderbook":
			svc.wsOBSvc.Unsubscribe(c)
		case "engine":
			svc.wsEngSvc.Unsubscribe(c)
		case "order":
			svc.wsOSvc.Unsubscribe(c)
		case "trade":
			svc.wsTradeSvc.Unsubscribe(c)
		case "quote":
			svc.wsOBSvc.UnsubscribeQuote(c)
		case "book":
			svc.wsOBSvc.UnsubscribeBook(c)
		}
	}
}

func (svc wsHandler) UnsubscribeAllHandler(input interface{}, c *ws.Client) {
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

	svc.wsTradeSvc.Unsubscribe(c)
	svc.wsOBSvc.UnsubscribeQuote(c)
	svc.wsOBSvc.UnsubscribeBook(c)

	protocol.SendSuccessMsg(connKey, "ok")
}

func (svc wsHandler) UnsubscribeAllHandlerPrivate(input interface{}, c *ws.Client) {
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

func (svc wsHandler) SubscribeHandlerPrivate(input interface{}, c *ws.Client) {
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

	protocol.SendSuccessMsg(connKey, msg.Params.Channels)

	for _, channel := range msg.Params.Channels {
		s := strings.Split(channel, ".")
		switch s[1] {
		case "orders":
			svc.wsOSvc.SubscribeUserOrder(c, channel, claim.UserID)
		case "trades":
			svc.wsTradeSvc.SubscribeUserTrades(c, channel, claim.UserID)
		case "changes":
			svc.wsOBSvc.SubscribeUserChange(c, channel, claim.UserID)
		}
	}

}

func (svc wsHandler) UnsubscribeHandlerPrivate(input interface{}, c *ws.Client) {
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

func (svc wsHandler) GetInstruments(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetInstrumentsParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
	}

	result := svc.wsOSvc.GetInstruments(context.TODO(), deribitModel.DeribitGetInstrumentsRequest{
		Currency: msg.Params.Currency,
		Expired:  msg.Params.Expired,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (svc wsHandler) GetOrderBook(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetOrderBookParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := svc.wsOBSvc.GetOrderBook(context.TODO(), deribitModel.DeribitGetOrderBookRequest{
		InstrumentName: msg.Params.InstrumentName,
		Depth:          msg.Params.Depth,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (svc wsHandler) GetLastTradesByInstrument(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetLastTradesByInstrumentParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := svc.wsOBSvc.GetLastTradesByInstrument(context.TODO(), deribitModel.DeribitGetLastTradesByInstrumentRequest{
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
		msg.Params.InstrumentName,
		deribitModel.DeribitGetUserTradesByOrderRequest{
			OrderId: msg.Params.OrderId,
			Sorting: msg.Params.Sorting,
		},
	)

	protocol.SendSuccessMsg(connKey, res)
}

func (svc wsHandler) GetIndexPrice(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetIndexPriceParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := svc.wsOBSvc.GetIndexPrice(context.TODO(), deribitModel.DeribitGetIndexPriceRequest{
		IndexName: msg.Params.IndexName,
	})

	protocol.SendSuccessMsg(connKey, result)
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
