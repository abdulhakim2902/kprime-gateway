package controller

import (
	"context"
	"errors"
	"fmt"
	deribitModel "gateway/internal/deribit/model"
	authService "gateway/internal/user/service"
	userType "gateway/internal/user/types"
	"gateway/pkg/constant"
	"gateway/pkg/hmac"
	"gateway/pkg/middleware"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"gateway/pkg/ws"
	"os"
	"strings"
	"time"

	"github.com/Undercurrent-Technologies/kprime-utilities/config/types"
	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"
)

// @title K-Prime Gateway API Documentation
// @version API Version 2
// @description Welcome to the K-Prime API Documentation! You can use our API to access K-Prime API endpoints, which can get informationin our database. We have language bindings in Shell! You can view code examples in the dark area to the right

func (handler *wsHandler) RegisterPublic() {
	ws.RegisterChannel("public/auth", middleware.MiddlewaresWrapper(handler.auth, middleware.RateLimiterWs))
	ws.RegisterChannel("public/subscribe", middleware.MiddlewaresWrapper(handler.publicSubscribe, middleware.RateLimiterWs))
	ws.RegisterChannel("public/unsubscribe", middleware.MiddlewaresWrapper(handler.publicUnsubscribe, middleware.RateLimiterWs))
	ws.RegisterChannel("public/unsubscribe_all", middleware.MiddlewaresWrapper(handler.publicUnsubscribeAll, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_last_trades_by_instrument", middleware.MiddlewaresWrapper(handler.getLastTradesByInstrument, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_index_price", middleware.MiddlewaresWrapper(handler.getIndexPrice, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_delivery_prices", middleware.MiddlewaresWrapper(handler.getDeliveryPrices, middleware.RateLimiterWs))
	ws.RegisterChannel("public/set_heartbeat", middleware.MiddlewaresWrapper(handler.setHeartbeat, middleware.RateLimiterWs))
	ws.RegisterChannel("public/test", middleware.MiddlewaresWrapper(handler.test, middleware.RateLimiterWs))
	ws.RegisterChannel("public/get_time", middleware.MiddlewaresWrapper(handler.publicGetTime, middleware.RateLimiterWs))
}

func validateSignatureAuth(params userType.AuthParams, connKey string) {
	if params.ClientID == "" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("client_id is a required field"))
		return
	}

	if params.Signature == "" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("signature is a required field"))
		return
	}

	if params.Timestamp == "" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("timestamp is a required field"))
		return
	}

	if params.GrantType == "" {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("grant_type is a required field"))
		return
	}
}

// auth asyncApi
// @summary This endpoint is use for login and get the access_token.
// @description endpoint: `ws://localhost:8080/ws/api/v2` with method public/auth
// @payload types.AuthParams
// @x-response types.AuthResponse
// @auth public
// @queue public/auth
// @method auth
// @tags public auth
// @contentType application/json
func (svc *wsHandler) auth(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[userType.AuthParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			c.SendInvalidRequestMessage(err)
		}
		return
	}
	// Add timeout
	go protocol.TimeOutProtocol(connKey)

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
		validateSignatureAuth(msg.Params, connKey)
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			c.SendInvalidRequestMessage(err)
		}
		return
	}

	// Add timeout
	go protocol.TimeOutProtocol(connKey)

	const t = true
	method := map[string]bool{"orderbook": t, "order": t, "trade": t, "trades": t, "quote": false, "book": t, "deribit_price_index": false, "ticker": t}
	interval := map[string]bool{"raw": t, "100ms": t, "agg2": t}
	for _, channel := range msg.Params.Channels {
		s := strings.Split(channel, ".")
		if len(s) == 0 {
			err := errors.New(constant.INVALID_CHANNEL)
			c.SendInvalidRequestMessage(err)
			return
		}

		if s[0] == "deribit_price_index" {
			if !types.Pair(s[1]).IsValid() {
				err := errors.New(constant.INVALID_INDEX_NAME)
				c.SendInvalidRequestMessage(err)
				return
			}
		} else {
			_, err = utils.ParseInstruments(s[1], false)
			if err != nil {
				protocol.SendValidationMsg(connKey,
					validation_reason.INVALID_PARAMS, err)
				return
			}
		}

		val, ok := method[s[0]]
		if !ok {
			err := errors.New(constant.INVALID_CHANNEL)
			c.SendInvalidRequestMessage(err)
			return
		}

		if val {
			if len(s) < 3 {
				err := errors.New(constant.INVALID_INTERVAL)
				c.SendInvalidRequestMessage(err)
				return
			}
			if _, ok := interval[s[2]]; !ok {
				err := errors.New(constant.INVALID_INTERVAL)
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			c.SendInvalidRequestMessage(err)
		}
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			c.SendInvalidRequestMessage(err)
		}
		return
	}

	svc.wsTradeSvc.Unsubscribe(c)
	svc.wsOBSvc.UnsubscribeQuote(c)
	svc.wsOBSvc.UnsubscribeBook(c)
	svc.wsRawPriceSvc.Unsubscribe(c)

	protocol.SendSuccessMsg(connKey, "ok")
}

func (svc *wsHandler) getLastTradesByInstrument(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[deribitModel.GetLastTradesByInstrumentParams]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	_, connKey, reason, err := requestHelper(msg.Id, msg.Method, nil, c)
	if err != nil {
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			c.SendInvalidRequestMessage(err)
		}
		return
	}
	// Add timeout
	go protocol.TimeOutProtocol(connKey)

	instruments, err := utils.ParseInstruments(msg.Params.InstrumentName, false)
	if err != nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, err)
		return
	}

	if instruments == nil {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid instrument_name"))
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			c.SendInvalidRequestMessage(err)
		}
		return
	}
	// Add timeout
	go protocol.TimeOutProtocol(connKey)

	if types.Pair(msg.Params.IndexName).IsValid() == false {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New(constant.INVALID_INDEX_NAME))
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			c.SendInvalidRequestMessage(err)
		}
		return
	}
	// Add timeout
	go protocol.TimeOutProtocol(connKey)

	if types.Pair(msg.Params.IndexName).IsValid() == false {
		protocol.SendValidationMsg(connKey,
			validation_reason.INVALID_PARAMS, errors.New("invalid index_name"))
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			c.SendInvalidRequestMessage(err)
		}
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
		if connKey != "" {
			protocol.SendValidationMsg(connKey, *reason, err)
		} else {
			c.SendInvalidRequestMessage(err)
		}
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

func (svc *wsHandler) publicGetTime(input interface{}, c *ws.Client) {
	var msg deribitModel.RequestDto[interface{}]
	if err := utils.UnmarshalAndValidateWS(input, &msg); err != nil {
		c.SendInvalidRequestMessage(err)
		return
	}

	now := time.Now().UnixMilli()
	c.Send(ws.WebsocketResponseMessage{
		JSONRPC: "2.0",
		ID:      msg.Id,
		Result:  now,
	})
}
