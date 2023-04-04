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
}

func NewWebsocketHandler(r *gin.Engine, authSvc service.IAuthService, deribitSvc deribitService.IDeribitService, wsOBSvc wsService.IwsOrderbookService) {
	handler := &wsHandler{
		authSvc:    authSvc,
		deribitSvc: deribitSvc,
		wsOBSvc:    wsOBSvc,
	}
	r.Use(cors.AllowAll())

	r.GET("/socket", ws.ConnectionEndpoint)

	ws.RegisterChannel("/public/auth", handler.PublicAuth)
	ws.RegisterChannel("/private/buy", handler.PrivateBuy)

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

	c.SendMessage(gin.H{"access_token": signedToken})
	return
}

func (svc wsHandler) PrivateBuy(input interface{}, c *ws.Client) {
	type Req struct {
		AccessToken    string  `json:"access_token"`
		InstrumentName string  `json:"instrument_name"`
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
	res, err := svc.deribitSvc.DeribitParseBuy(context.TODO(), JWTData.UserID, deribitModel.DeribitRequest{
		InstrumentName: msg.InstrumentName,
		Amount:         msg.Amount,
		Type:           msg.Type,
		Price:          msg.Price,
	})

	//register order connection

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
		fmt.Println(channel)
		s := strings.Split(channel, ".")
		switch s[0] {
		case "orderbook":
			svc.wsOBSvc.Unsubscribe(c)
		}
	}
}

// func hn(input interface{}, c *ws.Client) {
// 	fmt.Println("hn")
// 	fmt.Println(c)

// 	msg := &ws.WebsocketEvent{}

// 	ws.RegisterOrderConnection("aa", c)

// 	bytes, _ := json.Marshal(input)
// 	if err := json.Unmarshal(bytes, &msg); err != nil {
// 		fmt.Println("Error")
// 		c.SendMessage(ws.OrderChannel, "ERROR", err.Error())
// 	}

// 	ws.SendOrderMessage("ERROR", "aa", errors.New("Account is blocked2"))

// 	c.SendMessage(errors.New("Account is blocked"))
// 	fmt.Println(msg.Type)
// }

// func handler(c *gin.Context) {
// 	var upgrader = websocket.Upgrader{
// 		ReadBufferSize:  1024,
// 		WriteBufferSize: 1024,
// 		CheckOrigin: func(r *http.Request) bool {
// 			return true
// 		},
// 	}
// 	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	defer ws.Close()

// 	for {
// 		//Read Message from client
// 		mt, message, err := ws.ReadMessage()
// 		fmt.Println(mt)
// 		fmt.Println(message)
// 		if err != nil {
// 			fmt.Println(err)
// 			break
// 		}
// 		//If client message is ping will return pong
// 		if string(message) == "ping" {
// 			message = []byte("pong")
// 		}
// 		//Response message to client
// 		err = ws.WriteMessage(mt, message)
// 		if err != nil {
// 			fmt.Println(err)
// 			break
// 		}
// 	}

// }
