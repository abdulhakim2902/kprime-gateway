package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gateway/internal/auth/model"
	"gateway/internal/auth/service"
	deribitModel "gateway/internal/deribit/model"
	deribitService "gateway/internal/deribit/service"
	engService "gateway/internal/ws/engine/service"
	wsService "gateway/internal/ws/service"

	cors "github.com/rs/cors/wrapper/gin"

	"gateway/pkg/ws"

	"github.com/gin-gonic/gin"
)

type wsHandler struct {
	authSvc    service.IAuthService
	deribitSvc deribitService.IDeribitService
	wsOBSvc    wsService.IwsOrderbookService
	wsOSvc     wsService.IwsOrderService
	wsEngSvc   engService.IwsEngineService
	wsTradeSvc wsService.IwsTradeService
}

func NewWebsocketHandler(
	r *gin.Engine,
	authSvc service.IAuthService,
	deribitSvc deribitService.IDeribitService,
	wsOBSvc wsService.IwsOrderbookService,
	wsEngSvc engService.IwsEngineService,
	wsOSvc wsService.IwsOrderService,
	wsTradeSvc wsService.IwsTradeService,
) {
	handler := &wsHandler{
		authSvc:    authSvc,
		deribitSvc: deribitSvc,
		wsOBSvc:    wsOBSvc,
		wsEngSvc:   wsEngSvc,
		wsOSvc:     wsOSvc,
		wsTradeSvc: wsTradeSvc,
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

	ws.RegisterChannel("public/subscribe", handler.SubscribeHandler)
	ws.RegisterChannel("public/unsubscribe", handler.UnsubscribeHandler)

	ws.RegisterChannel("public/get_instruments", handler.GetInstruments)
}

func (svc wsHandler) PublicAuth(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())
	type Params struct {
		GrantType    string `json:"grant_type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}

	type WebsocketAuth struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &WebsocketAuth{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        "",
		})
		return
	}

	signedToken, err := svc.authSvc.APILogin(context.TODO(), model.APILoginRequest{
		APIKey:    msg.Params.ClientID,
		APISecret: msg.Params.ClientSecret,
	})
	if err != nil {
		fmt.Println(err)
		c.SendMessage(gin.H{"err": err.Error()}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	c.SendMessage(gin.H{"access_token": signedToken}, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
	return
}

func (svc wsHandler) PrivateBuy(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken    string  `json:"access_token"`
		InstrumentName string  `json:"instrument_name"`
		Amount         float64 `json:"amount"`
		Type           string  `json:"type"`
		Price          float64 `json:"price"`
		TimeInForce    string  `json:"time_in_force"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        "",
		})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        "",
		})
		return
	}

	ID := strconv.FormatUint(msg.Id, 10) + "-" + JWTData.UserID
	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)

	if !duplicateRpcID {
		c.SendMessage(gin.H{"err": errorMessage}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        JWTData.UserID,
		})
		return
	}

	// TODO: Validation

	// Parse the Deribit BUY
	_, err = svc.deribitSvc.DeribitParseBuy(context.TODO(), JWTData.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
	})

	// register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)

	// c.SendMessage(res)
	return
}

func (svc wsHandler) PrivateSell(input interface{}, c *ws.Client) {
	requestedTime := uint64(time.Now().UnixMicro())

	type Params struct {
		AccessToken    string  `json:"access_token"`
		InstrumentName string  `json:"instrument_name"`
		Amount         float64 `json:"amount"`
		Type           string  `json:"type"`
		Price          float64 `json:"price"`
		TimeInForce    string  `json:"time_in_force"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     uint64 `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        "",
		})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        JWTData.UserID,
		})
		return
	}

	ID := strconv.FormatUint(msg.Id, 10) + "-" + JWTData.UserID

	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)

	if !duplicateRpcID {
		c.SendMessage(gin.H{"err": errorMessage}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        JWTData.UserID,
		})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseSell(context.TODO(), JWTData.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
		TimeInForce:    msg.Params.TimeInForce,
	})

	// register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)

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
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	ID := strconv.FormatUint(msg.Id, 10) + "-" + JWTData.UserID

	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)

	if !duplicateRpcID {
		c.SendMessage(gin.H{"err": errorMessage}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        JWTData.UserID,
		})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseEdit(context.TODO(), JWTData.UserID, deribitModel.DeribitEditRequest{
		Id:      msg.Params.Id,
		Price:   msg.Params.Price,
		Amount:  msg.Params.Amount,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})

	// register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)
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
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	ID := strconv.FormatUint(msg.Id, 10) + "-" + JWTData.UserID

	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)

	if !duplicateRpcID {
		c.SendMessage(gin.H{"err": errorMessage}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        JWTData.UserID,
		})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseCancel(context.TODO(), JWTData.UserID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.Id,
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})

	// register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)
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
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	ID := strconv.FormatUint(msg.Id, 10) + "-" + JWTData.UserID

	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)

	if !duplicateRpcID {
		c.SendMessage(gin.H{"err": errorMessage}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        JWTData.UserID,
		})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	res, err := svc.deribitSvc.DeribitCancelByInstrument(context.TODO(), JWTData.UserID, deribitModel.DeribitCancelByInstrumentRequest{
		InstrumentName: msg.Params.InstrumentName,
		ClOrdID:        strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	//register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)
	c.SendMessage(map[string]interface{}{
		"userId":   res.UserId,
		"clientId": res.ClientId,
		"side":     res.Side,
	}, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
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
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

	ID := strconv.FormatUint(msg.Id, 10) + "-" + JWTData.UserID

	duplicateRpcID, errorMessage := c.RegisterRequestRpcIDS(ID, requestedTime)

	if !duplicateRpcID {
		c.SendMessage(gin.H{"err": errorMessage}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
			UserID:        JWTData.UserID,
		})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	res, err := svc.deribitSvc.DeribitParseCancelAll(context.TODO(), JWTData.UserID, deribitModel.DeribitCancelAllRequest{
		ClOrdID: strconv.FormatUint(msg.Id, 10),
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	// register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)
	c.SendMessage(map[string]interface{}{
		"userId":   res.UserId,
		"clientId": res.ClientId,
		"side":     res.Side,
	}, ws.SendMessageParams{
		ID:            msg.Id,
		RequestedTime: requestedTime,
	})
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
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
		return
	}

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
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
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
		}

	}
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
		c.SendMessage(gin.H{"err": err}, ws.SendMessageParams{
			ID:            msg.Id,
			RequestedTime: requestedTime,
		})
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
