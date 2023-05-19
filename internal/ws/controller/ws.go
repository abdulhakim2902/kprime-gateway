package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/pkg/utils"
	"strconv"
	"strings"
	"time"

	deribitModel "gateway/internal/deribit/model"
	"gateway/internal/repositories"
	userType "gateway/internal/user/types"

	deribitService "gateway/internal/deribit/service"
	authService "gateway/internal/user/service"
	engService "gateway/internal/ws/engine/service"
	"gateway/internal/ws/helpers"
	wsService "gateway/internal/ws/service"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/models/order"
	"git.devucc.name/dependencies/utilities/types"
	"git.devucc.name/dependencies/utilities/types/validation_reason"
	"github.com/go-playground/validator/v10"
	cors "github.com/rs/cors/wrapper/gin"

	"gateway/pkg/ws"

	"github.com/gin-gonic/gin"
)

type wsHandler struct {
	authSvc    authService.IAuthService
	deribitSvc deribitService.IDeribitService
	wsOBSvc    wsService.IwsOrderbookService
	wsOSvc     wsService.IwsOrderService
	wsEngSvc   engService.IwsEngineService
	wsTradeSvc wsService.IwsTradeService

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
	userRepo *repositories.UserRepository,
) {
	handler := &wsHandler{
		authSvc:    authSvc,
		deribitSvc: deribitSvc,
		wsOBSvc:    wsOBSvc,
		wsEngSvc:   wsEngSvc,
		wsOSvc:     wsOSvc,
		wsTradeSvc: wsTradeSvc,
		userRepo:   userRepo,
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
	ws.RegisterChannel("private/get_user_trades_by_instrument", handler.PrivateGetUserTradesByInstrument)
	ws.RegisterChannel("private/get_open_orders_by_instrument", handler.PrivateGetOpenOrdersByInstrument)
	ws.RegisterChannel("private/get_order_history_by_instrument", handler.PrivateGetOrderHistoryByInstrument)

	ws.RegisterChannel("public/subscribe", handler.SubscribeHandler)
	ws.RegisterChannel("public/unsubscribe", handler.UnsubscribeHandler)

	ws.RegisterChannel("public/get_instruments", handler.GetInstruments)
	ws.RegisterChannel("public/get_order_book", handler.GetOrderBook)
}

func (svc wsHandler) PublicAuth(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())
	type Params struct {
		GrantType    string `json:"grant_type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`
	}

	type WebsocketAuth struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &WebsocketAuth{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	var res any
	var err error

	switch msg.Params.GrantType {
	case "client_credentials":
		payload := userType.AuthRequest{
			APIKey:    msg.Params.ClientID,
			APISecret: msg.Params.ClientSecret,
		}

		if payload.APIKey == "" || payload.APISecret == "" {
			helpers.SendValidationResponse(c,
				validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, nil)
			return
		}

		res, err = svc.authSvc.Login(context.TODO(), payload, c)
		if err != nil {
			if strings.Contains(err.Error(), "invalid credential") {
				helpers.SendValidationResponse(c,
					validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, nil)
				return
			}

			fmt.Println(err)
			helpers.SendValidationResponse(c,
				validation_reason.OTHER, msg.Id, requestedTime, nil, nil)
			return
		}
	case "refresh_token":
		if msg.Params.RefreshToken == "" {
			reason := "refresh_token is required"
			helpers.SendValidationResponse(c,
				validation_reason.INVALID_PARAMS, msg.Id, requestedTime, nil, &reason)
			return
		}

		claim, err := svc.authSvc.ClaimJWT(msg.Params.RefreshToken, c)
		if err != nil {
			reason := err.Error()
			helpers.SendValidationResponse(c,
				validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
			return
		}

		res, err = svc.authSvc.RefreshToken(context.TODO(), claim, c)
		if err != nil {
			fmt.Println(err)
			helpers.SendValidationResponse(c,
				validation_reason.OTHER, msg.Id, requestedTime, nil, nil)
			return
		}
	}

	c.SendMessage(res, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
}

func (svc wsHandler) PrivateBuy(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Req struct {
		Params deribitModel.RequestParams `json:"params"`
		Id     uint64                     `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}
	// Check the Access Token
	claim, err := svc.authSvc.ClaimJWT(msg.Params.AccessToken, c)
	if err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)
	if !duplicateRpcID {
		helpers.SendValidationResponse(c,
			validation_reason.DUPLICATED_REQUEST_ID, msg.Id, requestedTime, &claim.UserID, &errorMessage)
		return
	}

	user, err := svc.userRepo.FindById(context.TODO(), claim.UserID)
	if err != nil {
		fmt.Println("userRepo.FindById:", err)

		helpers.SendValidationResponse(c,
			validation_reason.OTHER, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	var typeInclusions []order.TypeInclusions
	userHasOrderType := false
	for _, orderType := range user.OrderTypes {
		if strings.ToLower(orderType.Name) == strings.ToLower(string(msg.Params.Type)) {
			userHasOrderType = true
		}

		typeInclusions = append(typeInclusions, order.TypeInclusions{
			Name: orderType.Name,
		})
	}

	if !userHasOrderType {
		helpers.SendValidationResponse(c,
			validation_reason.ORDER_TYPE_NO_MATCH, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	var orderExclusions []order.OrderExclusion
	for _, item := range user.OrderExclusions {
		orderExclusions = append(orderExclusions, order.OrderExclusion{
			UserID: item.UserID,
		})
	}

	// Parse the Deribit BUY
	_, err = svc.deribitSvc.DeribitRequest(context.TODO(), claim.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.BUY,

		OrderExclusions: orderExclusions,
		TypeInclusions:  typeInclusions,
	})
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		helpers.SendValidationResponse(c,
			validation_reason.OTHER, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(ID, c)

	// c.SendMessage(res)
	return
}

func (svc wsHandler) PrivateSell(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Req struct {
		Params deribitModel.RequestParams `json:"params"`
		Id     uint64                     `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	// Check the Access Token
	claim, err := svc.authSvc.ClaimJWT(msg.Params.AccessToken, c)
	if err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)
	if !duplicateRpcID {
		helpers.SendValidationResponse(c,
			validation_reason.DUPLICATED_REQUEST_ID, msg.Id, requestedTime, &claim.UserID, &errorMessage)
		return
	}

	user, err := svc.userRepo.FindById(context.TODO(), claim.UserID)
	if err != nil {
		fmt.Println("userRepo.FindById:", err)

		helpers.SendValidationResponse(c,
			validation_reason.OTHER, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	var typeInclusions []order.TypeInclusions
	userHasOrderType := false
	for _, orderType := range user.OrderTypes {
		if strings.ToLower(orderType.Name) == strings.ToLower(string(msg.Params.Type)) {
			userHasOrderType = true
		}

		typeInclusions = append(typeInclusions, order.TypeInclusions{
			Name: orderType.Name,
		})
	}

	if !userHasOrderType {
		helpers.SendValidationResponse(c,
			validation_reason.ORDER_TYPE_NO_MATCH, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	var orderExclusions []order.OrderExclusion
	for _, item := range user.OrderExclusions {
		orderExclusions = append(orderExclusions, order.OrderExclusion{
			UserID: item.UserID,
		})
	}

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitRequest(context.TODO(), claim.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
		Label:          msg.Params.Label,
		Side:           types.SELL,

		OrderExclusions: orderExclusions,
		TypeInclusions:  typeInclusions,
	})
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		helpers.SendValidationResponse(c,
			validation_reason.OTHER, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(ID, c)

	// c.SendMessage(res)
	return
}

func (svc wsHandler) PrivateEdit(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken string  `json:"access_token"`
		Id          string  `json:"id"`
		Amount      float64 `json:"amount"`
		Price       float64 `json:"price"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	// Check the Access Token
	claim, err := svc.authSvc.ClaimJWT(msg.Params.AccessToken, c)
	if err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)
	if !duplicateRpcID {
		helpers.SendValidationResponse(c,
			validation_reason.DUPLICATED_REQUEST_ID, msg.Id, requestedTime, &claim.UserID, &errorMessage)
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
		logs.Log.Error().Err(err).Msg("")

		helpers.SendValidationResponse(c,
			validation_reason.OTHER, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(ID, c)
	return
}

func (svc wsHandler) PrivateCancel(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken string `json:"access_token"`
		Id          string `json:"id"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	// Check the Access Token
	claim, err := svc.authSvc.ClaimJWT(msg.Params.AccessToken, c)
	if err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)
	if !duplicateRpcID {
		helpers.SendValidationResponse(c,
			validation_reason.DUPLICATED_REQUEST_ID, msg.Id, requestedTime, &claim.UserID, &errorMessage)
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseCancel(context.TODO(), claim.UserID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.Id,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		helpers.SendValidationResponse(c,
			validation_reason.OTHER, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(ID, c)
	return
}

func (svc wsHandler) PrivateCancelByInstrument(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken    string `json:"access_token"`
		InstrumentName string `json:"instrument_name"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	// Check the Access Token
	claim, err := svc.authSvc.ClaimJWT(msg.Params.AccessToken, c)
	if err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)
	if !duplicateRpcID {
		helpers.SendValidationResponse(c,
			validation_reason.DUPLICATED_REQUEST_ID, msg.Id, requestedTime, &claim.UserID, &errorMessage)
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitCancelByInstrument(context.TODO(), claim.UserID, deribitModel.DeribitCancelByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		helpers.SendValidationResponse(c,
			validation_reason.OTHER, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	//register order connection
	ws.RegisterOrderConnection(ID, c)
	// c.SendMessage(map[string]interface{}{
	// 	"userId":   res.UserId,
	// 	"clientId": res.ClientId,
	// 	"side":     res.Side,
	// }, ws.SendMessageParams{
	// 	ID:            msg.Id,
	// 	RequestedTime: requestedTime,
	// })
}

func (svc wsHandler) PrivateCancelAll(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken string `json:"access_token"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	// Check the Access Token
	claim, err := svc.authSvc.ClaimJWT(msg.Params.AccessToken, c)
	if err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
		return
	}

	ID := utils.GetKeyFromIdUserID(msg.Id, claim.UserID)
	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)
	if !duplicateRpcID {
		helpers.SendValidationResponse(c,
			validation_reason.DUPLICATED_REQUEST_ID, msg.Id, requestedTime, &claim.UserID, &errorMessage)
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseCancelAll(context.TODO(), claim.UserID, deribitModel.DeribitCancelAllRequest{
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		helpers.SendValidationResponse(c,
			validation_reason.OTHER, msg.Id, requestedTime, &claim.UserID, nil)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(ID, c)
	// c.SendMessage(map[string]interface{}{
	// 	"userId":   res.UserId,
	// 	"clientId": res.ClientId,
	// 	"side":     res.Side,
	// }, ws.SendMessageParams{
	// 	ID:            msg.Id,
	// 	RequestedTime: requestedTime,
	// })
}

func (svc wsHandler) SubscribeHandler(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		Channels []string `json:"channels"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	c.SendMessage(msg.Params.Channels, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})

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
		case "quote":
			svc.wsOBSvc.SubscribeQuote(c, s[1])
		case "book":
			svc.wsOBSvc.SubscribeBook(c, channel)
		}

	}
}

func (svc wsHandler) UnsubscribeHandler(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		Channels []string `json:"channels"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

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
		}

	}

	c.SendMessage(msg.Params.Channels, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
}

func (svc wsHandler) GetInstruments(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken string `json:"accessToken"`
		Currency    string `json:"currency"`
		Expired     bool   `json:"expired"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	if msg.Params.Currency == "" {
		c.SendMessage(gin.H{"err": "Please provide currency"}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	result := svc.wsOSvc.GetInstruments(context.TODO(), deribitModel.DeribitGetInstrumentsRequest{
		Currency: msg.Params.Currency,
		Expired:  msg.Params.Expired,
	})

	c.SendMessage(result, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
	return
}

func (svc wsHandler) GetOrderBook(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		InstrumentName string `json:"instrument_name"`
		Depth          int64  `json:"depth"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	if msg.Params.InstrumentName == "" {
		reason := "Please provide instrument_name"
		helpers.SendValidationResponse(c,
			validation_reason.INVALID_PARAMS, msg.Id, requestedTime, nil, &reason)
		return
	}

	result := svc.wsOBSvc.GetOrderBook(context.TODO(), deribitModel.DeribitGetOrderBookRequest{
		InstrumentName: msg.Params.InstrumentName,
		Depth:          msg.Params.Depth,
	})

	c.SendMessage(result, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
}

func (svc wsHandler) PrivateGetUserTradesByInstrument(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken string `json:"access_token" validate:"required"`

		InstrumentName string `json:"instrument_name" validate:"required"`
		Count          int    `json:"count"`
		StartTimestamp int64  `json:"start_timestamp"`
		EndTimestamp   int64  `json:"end_timestamp"`
		Sorting        string `json:"sorting"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	validate := validator.New()
	if err := validate.Struct(msg); err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.INVALID_PARAMS, msg.Id, requestedTime, nil, &reason)
		return
	}

	// Number of requested items, default - 10
	if msg.Params.Count <= 0 {
		msg.Params.Count = 10
	}

	claim, err := svc.authSvc.ClaimJWT(msg.Params.AccessToken, c)
	if err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
		return
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

	c.SendMessage(res, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
}

func (svc wsHandler) PrivateGetOpenOrdersByInstrument(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken    string `json:"access_token" validate:"required"`
		InstrumentName string `json:"instrument_name" validate:"required"`
		Type           string `json:"type"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	validate := validator.New()
	if err := validate.Struct(msg); err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.INVALID_PARAMS, msg.Id, requestedTime, nil, &reason)
		return
	}

	// type parameter
	if msg.Params.Type == "" {
		msg.Params.Type = "all"
	}

	claim, err := svc.authSvc.ClaimJWT(msg.Params.AccessToken, c)
	if err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
		return
	}

	res := svc.wsOSvc.GetOpenOrdersByInstrument(
		context.TODO(),
		claim.UserID,
		deribitModel.DeribitGetOpenOrdersByInstrumentRequest{
			InstrumentName: msg.Params.InstrumentName,
			Type:           msg.Params.Type,
		},
	)

	c.SendMessage(res, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
}

func (svc wsHandler) PrivateGetOrderHistoryByInstrument(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken     string `json:"access_token" validate:"required"`
		InstrumentName  string `json:"instrument_name" validate:"required"`
		Count           int    `json:"count"`
		Offset          int    `json:"offset"`
		IncludeOld      bool   `json:"include_old"`
		IncludeUnfilled bool   `json:"include_unfilled"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		fmt.Println(err)
		helpers.SendValidationResponse(c,
			validation_reason.PARSE_ERROR, msg.Id, requestedTime, nil, nil)
		return
	}

	validate := validator.New()
	if err := validate.Struct(msg); err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.INVALID_PARAMS, msg.Id, requestedTime, nil, &reason)
		return
	}

	// parameter default value
	if msg.Params.Count <= 0 {
		msg.Params.Count = 20
	}

	claim, err := svc.authSvc.ClaimJWT(msg.Params.AccessToken, c)
	if err != nil {
		reason := err.Error()
		helpers.SendValidationResponse(c,
			validation_reason.UNAUTHORIZED, msg.Id, requestedTime, nil, &reason)
		return
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

	c.SendMessage(res, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
}
