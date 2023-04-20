package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	r.GET("/socket", ws.ConnectionEndpoint)

	ws.RegisterChannel("/public/auth", handler.PublicAuth)
	ws.RegisterChannel("/private/buy", handler.PrivateBuy)
	ws.RegisterChannel("/private/sell", handler.PrivateSell)
	ws.RegisterChannel("/private/edit", handler.PrivateEdit)
	ws.RegisterChannel("/private/cancel", handler.PrivateCancel)

	ws.RegisterChannel("/public/subscribe", handler.SubscribeHandler)
	ws.RegisterChannel("/public/unsubscribe", handler.UnsubscribeHandler)
}

func (svc wsHandler) PublicAuth(input interface{}, c *ws.Client) {
	type Params struct {
		GrantType    string `json:"grant_type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}

	type WebsocketAuth struct {
		Params Params `json:"params"`
	}

	msg := &WebsocketAuth{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	signedToken, err := svc.authSvc.APILogin(context.TODO(), model.APILoginRequest{
		APIKey:    msg.Params.ClientID,
		APISecret: msg.Params.ClientSecret,
	})
	if err != nil {
		fmt.Println(err)
		c.SendMessage(gin.H{"err": err.Error()})
		return
	}

	c.SendMessage(gin.H{"accessToken": signedToken})
	return
}

func (svc wsHandler) PrivateBuy(input interface{}, c *ws.Client) {
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
		Id     string `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()})
		return
	}

	// TODO: Validation

	// Parse the Deribit BUY
	_, err = svc.deribitSvc.DeribitParseBuy(context.TODO(), JWTData.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        msg.Id,
		TimeInForce:    msg.Params.TimeInForce,
	})

	// register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)

	// c.SendMessage(res)
	return
}

func (svc wsHandler) PrivateSell(input interface{}, c *ws.Client) {
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
		Id     string `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseSell(context.TODO(), JWTData.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.Params.InstrumentName,
		Amount:         msg.Params.Amount,
		Type:           msg.Params.Type,
		Price:          msg.Params.Price,
		ClOrdID:        msg.Id,
		TimeInForce:    msg.Params.TimeInForce,
	})

	// register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)

	// c.SendMessage(res)
	return
}

func (svc wsHandler) PrivateEdit(input interface{}, c *ws.Client) {
	type Params struct {
		AccessToken string  `json:"access_token"`
		Id          string  `json:"id"`
		Amount      float64 `json:"amount"`
		Price       float64 `json:"price"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     string `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	res, err := svc.deribitSvc.DeribitParseEdit(context.TODO(), JWTData.UserID, deribitModel.DeribitEditRequest{
		Id:      msg.Params.Id,
		Price:   msg.Params.Price,
		Amount:  msg.Params.Amount,
		ClOrdID: msg.Id,
	})

	// register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)
	c.SendMessage(map[string]interface{}{
		"id":       res.Id,
		"userId":   res.UserId,
		"clientId": res.ClientId,
		"side":     res.Side,
		"price":    res.Price,
		"amount":   res.Amount,
	}, res.ClOrdID)
	return
}

func (svc wsHandler) PrivateCancel(input interface{}, c *ws.Client) {
	type Params struct {
		AccessToken string `json:"access_token"`
		Id          string `json:"id"`
	}

	type Req struct {
		Params Params `json:"params"`
		Id     string `json:"id"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.Params.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	res, err := svc.deribitSvc.DeribitParseCancel(context.TODO(), JWTData.UserID, deribitModel.DeribitCancelRequest{
		Id:      msg.Params.Id,
		ClOrdID: msg.Id,
	})

	// register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)
	c.SendMessage(map[string]interface{}{
		"id":       res.Id,
		"userId":   res.UserId,
		"clientId": res.ClientId,
		"side":     res.Side,
	}, res.ClOrdID)
	return
}

func (svc wsHandler) SubscribeHandler(input interface{}, c *ws.Client) {
	type Params struct {
		Channels []string `json:"channels"`
	}

	type Req struct {
		Params Params `json:"params"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
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
	type Params struct {
		Channels []string `json:"channels"`
	}

	type Req struct {
		Params Params `json:"params"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
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
