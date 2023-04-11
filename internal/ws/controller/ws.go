package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/auth/model"
	"gateway/internal/auth/service"
	deribitModel "gateway/internal/deribit/model"
	deribitService "gateway/internal/deribit/service"
	wsService "gateway/internal/ws/service"
	"strings"

	cors "github.com/rs/cors/wrapper/gin"

	"gateway/pkg/ws"

	"github.com/gin-gonic/gin"
)

type wsHandler struct {
	authSvc    service.IAuthService
	deribitSvc deribitService.IDeribitService
	wsOBSvc    wsService.IwsOrderbookService
	wsOSvc     wsService.IwsOrderService
}

func NewWebsocketHandler(r *gin.Engine, authSvc service.IAuthService, deribitSvc deribitService.IDeribitService, wsOBSvc wsService.IwsOrderbookService, wsOSvc wsService.IwsOrderService) {
	handler := &wsHandler{
		authSvc:    authSvc,
		deribitSvc: deribitSvc,
		wsOBSvc:    wsOBSvc,
		wsOSvc:     wsOSvc,
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
	type WebsocketAuth struct {
		GrantType    string `json:"grant_type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}

	msg := &WebsocketAuth{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	signedToken, err := svc.authSvc.APILogin(context.TODO(), model.APILoginRequest{
		APIKey:    msg.ClientID,
		APISecret: msg.ClientSecret,
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
	type Req struct {
		AccessToken    string  `json:"accessToken"`
		InstrumentName string  `json:"instrumentName"`
		Amount         float64 `json:"amount"`
		Type           string  `json:"type"`
		Price          float64 `json:"price"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()})
		return
	}

	// TODO: Validation

	// Parse the Deribit BUY
	_, err = svc.deribitSvc.DeribitParseBuy(context.TODO(), JWTData.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.InstrumentName,
		Amount:         msg.Amount,
		Type:           msg.Type,
		Price:          msg.Price,
	})

	//register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)

	// c.SendMessage(res)
	return
}

func (svc wsHandler) PrivateSell(input interface{}, c *ws.Client) {
	type Req struct {
		AccessToken    string  `json:"accessToken"`
		InstrumentName string  `json:"instrumentName"`
		Amount         float64 `json:"amount"`
		Type           string  `json:"type"`
		Price          float64 `json:"price"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	_, err = svc.deribitSvc.DeribitParseSell(context.TODO(), JWTData.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.InstrumentName,
		Amount:         msg.Amount,
		Type:           msg.Type,
		Price:          msg.Price,
	})

	//register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)

	// c.SendMessage(res)
	return
}

func (svc wsHandler) PrivateEdit(input interface{}, c *ws.Client) {
	type Req struct {
		OrderId        string  `json:"orderId"`
		AccessToken    string  `json:"accessToken"`
		InstrumentName string  `json:"instrumentName"`
		Amount         float64 `json:"amount"`
		Type           string  `json:"type"`
		Price          float64 `json:"price"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	res, err := svc.deribitSvc.DeribitParseEdit(context.TODO(), JWTData.UserID, deribitModel.DeribitEditRequest{
		OrderId:        msg.OrderId,
		InstrumentName: msg.InstrumentName,
		Amount:         msg.Amount,
		Type:           msg.Type,
		Price:          msg.Price,
	})

	//register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)

	c.SendMessage(res)
	return
}

func (svc wsHandler) PrivateCancel(input interface{}, c *ws.Client) {
	type Req struct {
		OrderId        string  `json:"orderId"`
		AccessToken    string  `json:"accessToken"`
		InstrumentName string  `json:"instrumentName"`
		Amount         float64 `json:"amount"`
		Type           string  `json:"type"`
		Price          float64 `json:"price"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	// Check the Access Token
	JWTData, err := svc.authSvc.JWTCheck(msg.AccessToken)
	if err != nil {
		c.SendMessage(gin.H{"err": err.Error()})
		return
	}

	// TODO: Validation

	// Parse the Deribit Sell
	res, err := svc.deribitSvc.DeribitParseCancel(context.TODO(), JWTData.UserID, deribitModel.DeribitCancelRequest{
		OrderId:        msg.OrderId,
		InstrumentName: msg.InstrumentName,
		Amount:         msg.Amount,
		Type:           msg.Type,
		Price:          msg.Price,
	})

	//register order connection
	ws.RegisterOrderConnection(JWTData.UserID, c)

	c.SendMessage(res)
	return
}

func (svc wsHandler) SubscribeHandler(input interface{}, c *ws.Client) {
	type Req struct {
		Channels []string `json:"channels"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	for _, channel := range msg.Channels {
		fmt.Println(channel)
		s := strings.Split(channel, ".")
		switch s[0] {
		case "orderbook":
			svc.wsOBSvc.Subscribe(c, s[1])
		case "order":
			svc.wsOSvc.Subscribe(c, s[1])
		}

	}
}

func (svc wsHandler) UnsubscribeHandler(input interface{}, c *ws.Client) {
	type Req struct {
		Channels []string `json:"channels"`
	}

	msg := &Req{}
	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		c.SendMessage(gin.H{"err": err})
		return
	}

	for _, channel := range msg.Channels {
		s := strings.Split(channel, ".")
		switch s[0] {
		case "orderbook":
			svc.wsOBSvc.Unsubscribe(c)
		case "order":
			svc.wsOSvc.Unsubscribe(c)
		}

	}
}
