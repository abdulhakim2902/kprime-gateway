package controller

import (
	"context"
	"errors"
	"fmt"
	deribitModel "gateway/internal/deribit/model"
	authService "gateway/internal/user/service"
	userType "gateway/internal/user/types"
	"gateway/pkg/hmac"
	"gateway/pkg/middleware"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"gateway/pkg/ws"
	"os"
	"strings"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
)

func (handler *wsHandler) RegisterPublic() {
	ws.RegisterChannel("public/auth", middleware.MiddlewaresWrapper(handler.auth, middleware.RateLimiterWs))
	ws.RegisterChannel("public/subscribe", middleware.MiddlewaresWrapper(handler.publicSubscribe, middleware.RateLimiterWs))
	ws.RegisterChannel("public/unsubscribe", middleware.MiddlewaresWrapper(handler.publicUnsubscribe, middleware.RateLimiterWs))
	ws.RegisterChannel("public/unsubscribe_all", middleware.MiddlewaresWrapper(handler.publicUnsubscribeAll, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_instruments", middleware.MiddlewaresWrapper(handler.getInstruments, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_last_trades_by_instrument", middleware.MiddlewaresWrapper(handler.getLastTradesByInstrument, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_order_book", middleware.MiddlewaresWrapper(handler.getOrderBook, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_index_price", middleware.MiddlewaresWrapper(handler.getIndexPrice, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_delivery_prices", middleware.MiddlewaresWrapper(handler.getDeliveryPrices, middleware.RateLimiterWs))
	ws.RegisterChannel("public/set_heartbeat", middleware.MiddlewaresWrapper(handler.setHeartbeat, middleware.RateLimiterWs))
	ws.RegisterChannel("public/test", middleware.MiddlewaresWrapper(handler.test, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_tradingview_chart_data", middleware.MiddlewaresWrapper(handler.publicGetTradingviewChartData, middleware.RateLimiterWs))
}

func (svc *wsHandler) auth(input interface{}, c *ws.Client) {
	type Params struct {
		GrantType    string `json:"grant_type" validate:"required"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`

		Signature string `json:"signature"`
		Timestamp string `json:"timestamp"`
		Nonce     string `json:"nonce"`
		Data      string `json:"data"`
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
	case "client_signature":
		sig := hmac.Signature{
			Ts:       msg.Params.Timestamp,
			Sig:      msg.Params.Signature,
			Nonce:    msg.Params.Nonce,
			ClientId: msg.Params.ClientID,
			Data:     msg.Params.Data,
		}

		res, user, err = svc.authSvc.LoginWithSignature(context.TODO(), sig)
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

func (svc *wsHandler) publicSubscribe(input interface{}, c *ws.Client) {
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

	const t = true
	method := map[string]bool{"orderbook": t, "engine": t, "order": t, "trade": t, "trades": t, "quote": false, "book": t, "deribit_price_index": false, "ticker": t}
	interval := map[string]bool{"raw": t, "100ms": t, "agg2": t}
	for _, channel := range msg.Params.Channels {
		s := strings.Split(channel, ".")
		if len(s) == 0 {
			err := errors.New("error invalid channel")
			c.SendInvalidRequestMessage(err)
			return
		}
		val, ok := method[s[0]]
		if !ok {
			err := errors.New("error invalid channel")
			c.SendInvalidRequestMessage(err)
			return
		}

		if val {
			if len(s) < 3 {
				err := errors.New("error invalid interval")
				c.SendInvalidRequestMessage(err)
				return
			}
			if _, ok := interval[s[2]]; !ok {
				err := errors.New("error invalid interval")
				c.SendInvalidRequestMessage(err)
				return
			}
		}
	}

	protocol.SendSuccessMsg(connKey, msg.Params.Channels)

	for _, channel := range msg.Params.Channels {
		s := strings.Split(channel, ".")

		if (s[0] == "trades" || s[0] == "book") && len(s) != 3 {
			reason := validation_reason.INVALID_PARAMS
			err := fmt.Errorf("unrecognize channel for '%s'", channel)
			protocol.SendValidationMsg(connKey, reason, err)
			return
		}

		switch s[0] {
		case "engine":
			svc.wsEngSvc.Subscribe(c, s[1])
		case "trades":
			svc.wsTradeSvc.SubscribeTrades(c, channel)
		case "quote":
			svc.wsOBSvc.SubscribeQuote(c, s[1])
		case "book":
			svc.wsOBSvc.SubscribeBook(c, channel, s[1], s[2])
		case "deribit_price_index":
			svc.wsRawPriceSvc.Subscribe(c, s[1])
		case "ticker":
			svc.wsOBSvc.SubscribeTicker(c, channel, s[1], s[2])
		default:
			reason := validation_reason.INVALID_PARAMS
			err := fmt.Errorf("unrecognize channel for '%s'", channel)
			protocol.SendValidationMsg(connKey, reason, err)
			return
		}
	}

	protocol.SendSuccessMsg(connKey, msg.Params.Channels)
}

func (svc *wsHandler) publicUnsubscribe(input interface{}, c *ws.Client) {
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

	for _, channel := range msg.Params.Channels {
		s := strings.Split(channel, ".")

		switch s[0] {
		case "engine":
			svc.wsEngSvc.Unsubscribe(c)
		case "quote":
			svc.wsOBSvc.UnsubscribeQuote(c)
		case "book":
			svc.wsOBSvc.UnsubscribeBook(c)
		default:
			reason := validation_reason.INVALID_PARAMS
			err := fmt.Errorf("unrecognize channel for '%s'", channel)
			protocol.SendValidationMsg(connKey, reason, err)
			return
		}
	}

	protocol.SendSuccessMsg(connKey, msg.Params.Channels)
}

func (svc *wsHandler) publicUnsubscribeAll(input interface{}, c *ws.Client) {
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
	svc.wsRawPriceSvc.Unsubscribe(c)

	protocol.SendSuccessMsg(connKey, "ok")
}

func (svc *wsHandler) getInstruments(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetInstrumentsParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
	}

	currency := map[string]bool{"BTC": true, "ETH": true, "USDC": true}
	if _, ok := currency[strings.ToUpper(msg.Params.Currency)]; !ok {
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

	result := svc.wsOSvc.GetInstruments(context.TODO(), deribitModel.DeribitGetInstrumentsRequest{
		Currency: msg.Params.Currency,
		Expired:  msg.Params.Expired,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (svc *wsHandler) getOrderBook(input interface{}, c *ws.Client) {
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

	instruments, _ := utils.ParseInstruments(msg.Params.InstrumentName)

	if instruments == nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("instrument not found"))
		return
	}

	result := svc.wsOBSvc.GetOrderBook(context.TODO(), deribitModel.DeribitGetOrderBookRequest{
		InstrumentName: msg.Params.InstrumentName,
		Depth:          msg.Params.Depth,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (svc *wsHandler) getLastTradesByInstrument(input interface{}, c *ws.Client) {
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

func (svc *wsHandler) getIndexPrice(input interface{}, c *ws.Client) {
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

func (svc *wsHandler) getDeliveryPrices(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.DeliveryPricesParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	result := svc.wsOBSvc.GetDeliveryPrices(context.TODO(), deribitModel.DeliveryPricesRequest{
		IndexName: msg.Params.IndexName,
		Offset:    msg.Params.Offset,
		Count:     msg.Params.Count,
	})

	protocol.SendSuccessMsg(connKey, result)
}

func (svc *wsHandler) setHeartbeat(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.SetHeartbeatParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	// parameter default value
	if msg.Params.Interval < 10 {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("interval must be 10 or greater"))
		return
	}
	go svc.wsEngSvc.SubscribeHeartbeat(c, connKey, msg.Params.Interval)

	protocol.SendSuccessMsg(connKey, "ok")
}

func (svc *wsHandler) test(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.TestParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

	go svc.wsEngSvc.AddHeartbeat(c)

	type Version struct {
		Version string `json:"version"`
	}

	version, exists := os.LookupEnv("APP_VERSION")
	if !exists {
		version = "1.0.0"
	}

	result := Version{
		Version: version,
	}

	protocol.SendSuccessMsg(connKey, result)
}

func (svc *wsHandler) publicGetTradingviewChartData(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetTradingviewChartDataRequest]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		protocol.SendValidationMsg(connKey, *reason, err)
		return
	}

}
