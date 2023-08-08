// Copyright (c) quickfixengine.org  All rights reserved.
//
// This file may be distributed under the terms of the quickfixengine.org
// license as defined by quickfixengine.org and appearing in the file
// LICENSE included in the packaging of this file.
//
// This file is provided AS IS with NO WARRANTY OF ANY KIND, INCLUDING
// THE WARRANTY OF DESIGN, MERCHANTABILITY AND FITNESS FOR A
// PARTICULAR PURPOSE.
//
// See http://www.quickfixengine.org/LICENSE for licensing information.
//
// Contact ask@quickfixengine.org if any conditions of this licensing
// are not clear to you.

package ordermatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	_deribitModel "gateway/internal/deribit/model"
	_deribitSvc "gateway/internal/deribit/service"
	"gateway/internal/engine/types"
	"gateway/internal/repositories"
	_wsSvc "gateway/internal/ws/service"
	"gateway/pkg/redis"
	"gateway/pkg/utils"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_mongo "gateway/pkg/mongo"

	_orderbookType "gateway/internal/orderbook/types"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/log"
	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	_utilitiesType "github.com/Undercurrent-Technologies/kprime-utilities/types"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix43/ordermasscancelreport"
	"github.com/quickfixgo/fix44/executionreport"
	"github.com/quickfixgo/fix44/marketdataincrementalrefresh"
	"github.com/quickfixgo/fix44/marketdatarequest"
	"github.com/quickfixgo/fix44/marketdatasnapshotfullrefresh"
	"github.com/quickfixgo/fix44/newordersingle"
	"github.com/quickfixgo/fix44/ordercancelreject"
	"github.com/quickfixgo/fix44/ordercancelreplacerequest"
	"github.com/quickfixgo/fix44/ordercancelrequest"
	"github.com/quickfixgo/fix44/ordermasscancelrequest"
	"github.com/quickfixgo/fix44/quotestatusrequest"
	"github.com/quickfixgo/fix44/securitylistrequest"
	"github.com/quickfixgo/fix44/tradecapturereportrequest"
	"github.com/quickfixgo/tag"
	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/quickfixgo/quickfix"
)

type XMessageSubscriber struct {
	sessiondID quickfix.SessionID
	secReq     string
}

type VMessageSubscriber struct {
	sessiondID     quickfix.SessionID
	InstrumentName string
	Bid            bool
	Ask            bool
	Trade          bool
}

type SubsManager struct {
	// Subscribing MsgType AD
	ADSubscriptions     map[string]map[quickfix.SessionID]bool
	ADSubscriptionsList map[quickfix.SessionID][]string

	// For quote status report
	ASubscriptions     map[string]map[quickfix.SessionID]bool
	ASubscriptionsList map[quickfix.SessionID][]string

	// For security list request (x)
	XSubscriptions     map[string]map[quickfix.SessionID]bool
	XSubscriptionsList map[quickfix.SessionID][]string

	// For market data request (V)
	VSubscriptions     map[string]map[quickfix.SessionID]bool
	VSubscriptionsList map[quickfix.SessionID][]string

	// Add more subscription here
	mu sync.Mutex
}

var subsManager = &SubsManager{
	ADSubscriptions:     make(map[string]map[quickfix.SessionID]bool),
	ADSubscriptionsList: make(map[quickfix.SessionID][]string),

	ASubscriptions:     make(map[string]map[quickfix.SessionID]bool),
	ASubscriptionsList: make(map[quickfix.SessionID][]string),

	XSubscriptions:     make(map[string]map[quickfix.SessionID]bool),
	XSubscriptionsList: make(map[quickfix.SessionID][]string),

	VSubscriptions:     make(map[string]map[quickfix.SessionID]bool),
	VSubscriptionsList: make(map[quickfix.SessionID][]string),

	mu: sync.Mutex{},
}

var userSession map[string]*quickfix.SessionID
var vMessageSubs []VMessageSubscriber
var xMessagesSubs []XMessageSubscriber

type Order struct {
	ID                   string          `json:"id" bson:"_id"`
	ClientOrderId        string          `json:"clOrdID" bson:"clOrdID"`
	InstrumentName       string          `json:"instrumentName" bson:"instrumentName"`
	Symbol               string          `json:"symbol" bson:"symbol"`
	SenderCompID         string          `json:"sender_comp_id" bson:"sender_comp_id"`
	TargetCompID         string          `json:"target_comp_id" bson:"target_comp_id"`
	UserID               string          `json:"userId" bson:"userId"`     // our own client id
	ClientID             string          `json:"clientId" bson:"clientId"` // party id, user from client
	Underlying           string          `json:"underlying" bson:"underlying"`
	ExpiryDate           string          `json:"expiryDate" bson:"expiryDate"`
	StrikePrice          float64         `json:"strikePrice" bson:"strikePrice"`
	Type                 enum.OrdType    `json:"type" bson:"type"`
	Side                 enum.Side       `json:"side" bson:"side"`
	Price                decimal.Decimal `json:"price" bson:"price"` //avgpx
	Amount               decimal.Decimal `json:"amount" bson:"amount"`
	FilledAmount         decimal.Decimal `json:"filledAmount" bson:"filledAmount"`
	Contracts            string          `json:"contracts" bson:"contracts"`
	Status               string          `json:"status" bson:"status"`
	CreatedAt            time.Time       `json:"createdAt" bson:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt" bson:"updatedAt"`
	insertTime           time.Time
	LastExecutedQuantity decimal.Decimal
	LastExecutedPrice    decimal.Decimal
}

type MarketDataResponse struct {
	InstrumentName string                   `json:"instrumentName"`
	Side           _utilitiesType.Side      `json:"side"`
	Contract       _utilitiesType.Contracts `json:"contract"`
	EntryType      enum.MDEntryType         `json:"entryType"`
	Price          float64                  `json:"price"`
	Amount         float64                  `json:"amount"`
	Date           string                   `json:"date"`
	Type           string                   `json:"type"`
	MakerID        string                   `json:"makerId"`
	TakerID        string                   `json:"takerId"`
	Status         string                   `json:"status"`
	UpdateType     string                   `json:"updateType"`
}

type Orderbook struct {
	InstrumentName string   `json:"instrumentName" bson:"instrumentName"`
	Bids           []*Order `json:"bids" bson:"bids"`
	Asks           []*Order `json:"asks" bson:"asks"`
}

// Application implements the quickfix.Application interface
type Application struct {
	*quickfix.MessageRouter
	execID int
	*repositories.UserRepository
	*repositories.OrderRepository
	*repositories.TradeRepository
	DeribitService _deribitSvc.IDeribitService
	WSService      _wsSvc.IwsOrderbookService
	redis          *redis.RedisConnectionPool
}

func newApplication(deribit _deribitSvc.IDeribitService, wsSvc _wsSvc.IwsOrderbookService) *Application {

	mongoDb, _ := _mongo.InitConnection(os.Getenv("MONGO_URL"))
	redis := redis.NewRedisConnectionPool(os.Getenv("REDIS_URL"))

	orderRepo := repositories.NewOrderRepository(mongoDb)
	tradeRepo := repositories.NewTradeRepository(mongoDb)
	userRepo := repositories.NewUserRepository(mongoDb)

	app := &Application{
		MessageRouter:   quickfix.NewMessageRouter(),
		UserRepository:  userRepo,
		OrderRepository: orderRepo,
		TradeRepository: tradeRepo,
		WSService:       wsSvc,
		DeribitService:  deribit,
		redis:           redis,
	}
	app.AddRoute(newordersingle.Route(app.onNewOrderSingle))
	app.AddRoute(ordercancelrequest.Route(app.onOrderCancelRequest))
	app.AddRoute(marketdatarequest.Route(app.onMarketDataRequest))
	app.AddRoute(ordercancelreplacerequest.Route(app.onOrderUpdateRequest))
	app.AddRoute(ordermasscancelrequest.Route(app.onOrderMassCancelRequest))
	app.AddRoute(securitylistrequest.Route(app.onSecurityListRequest))
	app.AddRoute(tradecapturereportrequest.Route(app.OnTradeCaptureReportRequest))
	app.AddRoute(quotestatusrequest.Route(app.OnQuoteStatusRequest))
	return app
}

// OnCreate implemented as part of Application interface
func (a Application) OnCreate(sessionID quickfix.SessionID) {}

// OnLogon implemented as part of Application interface
func (a Application) OnLogon(sessionID quickfix.SessionID) {}

// OnLogout implemented as part of Application interface
func (a Application) OnLogout(sessionID quickfix.SessionID) {
	// unsubscribe
	ADUnsubscribeAll(sessionID)
	AUnsubscribeAll(sessionID)
	XUnsubscribeAll(sessionID)
	VUnsubscribeAll(sessionID)
}

// ToAdmin implemented as part of Application interface
func (a Application) ToAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) {}

// ToApp implemented as part of Application interface
func (a Application) ToApp(msg *quickfix.Message, sessionID quickfix.SessionID) error {
	return nil
}

// FromAdmin implemented as part of Application interface
func (a Application) FromAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	if msg.IsMsgTypeOf(string(enum.MsgType_LOGON)) {
		var uname field.UsernameField
		var pwd field.PasswordField
		if err := msg.Body.Get(&pwd); err != nil {
			logs.Log.Err(err).Msg("Error getting password")
			return err
		}

		if err := msg.Body.Get(&uname); err != nil {
			logs.Log.Err(err).Msg("Error getting username")
			return err
		}

		user, err := a.UserRepository.FindByAPIKeyAndSecret(context.TODO(), uname.String(), pwd.String())
		if err != nil {
			logs.Log.Err(err).Msg("Error getting user")
			return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
		}

		if userSession == nil {
			userSession = make(map[string]*quickfix.SessionID)
		}

		userSession[user.ID.Hex()] = &sessionID
	}
	return nil
}

// FromApp implemented as part of Application interface, uses Router on incoming application messages
func (a *Application) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	return a.Route(msg, sessionID)
}

func (a *Application) onNewOrderSingle(msg newordersingle.NewOrderSingle, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	logs.Log.Info().Str("ordermatch", "onNewOrderSingle").Msg("")
	if userSession == nil {
		logs.Log.Err(fmt.Errorf("User not logged in")).Msg("User not logged in")
		return quickfix.NewMessageRejectError("User not logged in", 1, nil)
	}

	userId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			userId = i
		}
	}

	user, e := a.UserRepository.FindById(context.TODO(), userId)
	if e != nil {
		logs.Log.Err(e).Msg("Failed getting user")
		return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
	}

	clOrId, err := msg.GetClOrdID()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting clOrdId")
		return err
	}

	symbol, err := msg.GetSymbol()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting symbol")
		return err
	}

	side, err := msg.GetSide()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting side")
		return err
	}

	ordType, err := msg.GetOrdType()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting ordType")
		return err
	}

	price, err := msg.GetPrice()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting price")
		return err
	}

	orderQty, err := msg.GetOrderQty()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting orderQty")
		return err
	}

	var partyId quickfix.FIXString
	err = msg.GetField(tag.PartyID, &partyId)
	if err != nil {
		logs.Log.Err(err).Msg("Error getting partyId")
		return err
	}

	orderType := _utilitiesType.LIMIT
	if ordType == enum.OrdType_MARKET {
		orderType = _utilitiesType.MARKET
	}

	sideType := _utilitiesType.BUY
	if side == enum.Side_SELL {
		sideType = _utilitiesType.SELL
	}
	priceFloat, _ := strconv.ParseFloat(price.String(), 64)
	amountFloat, _ := strconv.ParseFloat(orderQty.String(), 64)

	tif, err := msg.GetTimeInForce()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting time in force")
		return err
	}

	tifType := _utilitiesType.GOOD_TIL_CANCELLED
	if tif == enum.TimeInForce_GOOD_TILL_CANCEL {
		tifType = _utilitiesType.GOOD_TIL_DAY
	} else if tif == enum.TimeInForce_IMMEDIATE_OR_CANCEL {
		tifType = _utilitiesType.IMMEDIATE_OR_CANCEL
	} else if tif == enum.TimeInForce_FILL_OR_KILL {
		tifType = _utilitiesType.FILL_OR_KILL
	}

	fmt.Println("debug tif", tif)
	fmt.Println("debug tifType", tifType)

	response, reason, r := a.DeribitService.DeribitRequest(context.TODO(), user.ID.Hex(), _deribitModel.DeribitRequest{
		ClientId:       partyId.String(),
		InstrumentName: symbol,
		ClOrdID:        clOrId,
		Type:           orderType,
		Side:           sideType,
		Price:          priceFloat,
		Amount:         amountFloat,
		TimeInForce:    tifType,
		MaxShow:        0.1,
		ReduceOnly:     false,
		PostOnly:       false,
	})

	if r != nil {
		if reason != nil {
			logs.Log.Err(r).Msg(fmt.Sprintf("Error placing order, %v", reason.String()))
			return quickfix.NewMessageRejectError(fmt.Sprintf("Error placing order, %v", reason.String()), 1, nil)
		}
		logs.Log.Err(r).Msg(fmt.Sprintf("Error placing order, %v", r.Error()))
		return quickfix.NewMessageRejectError(fmt.Sprintf("Error placing order, %v", r.Error()), 1, nil)
	}

	fmt.Println(response, reason, r)
	return nil
}

func (a *Application) onOrderUpdateRequest(msg ordercancelreplacerequest.OrderCancelReplaceRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	userId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			userId = i
		}
	}

	user, e := a.UserRepository.FindById(context.TODO(), userId)
	if e != nil {
		logs.Log.Err(e).Msg("Failed getting user")
		return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
	}

	price, err := msg.GetPrice()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting price")
		return err
	}

	ordType, err := msg.GetOrdType()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting ordType")
		return err
	}

	amount, err := msg.GetOrderQty()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting amount")
		return err
	}
	orderId, err := msg.GetOrderID()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting orderId")
		return err
	}

	var partyId quickfix.FIXString
	err = msg.GetField(tag.PartyID, &partyId)
	if err != nil {
		logs.Log.Err(err).Msg("Error getting partyId")
		return err
	}

	symbol, err := msg.GetSymbol()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting symbol")
		return err
	}

	amountFloat, _ := amount.Float64()
	priceFloat, _ := price.Float64()

	orderType := _utilitiesType.LIMIT
	if ordType == enum.OrdType_MARKET {
		orderType = _utilitiesType.MARKET
	}

	a.DeribitService.DeribitRequest(context.TODO(), user.ID.Hex(), _deribitModel.DeribitRequest{
		ID:             orderId,
		ClientId:       partyId.String(),
		InstrumentName: symbol,
		Type:           orderType,
		Side:           _utilitiesType.EDIT,
		Price:          priceFloat,
		Amount:         amountFloat,
	})

	return nil
}

func (a *Application) onOrderMassCancelRequest(msg ordermasscancelrequest.OrderMassCancelRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	logs.Log.Info().Str("ordermatch", "onOrderMassCancelRequest").Msg("")
	symbol, err := msg.GetSymbol()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting symbol")
	}

	userId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			userId = i
		}
	}

	clOrdID, err := msg.GetClOrdID()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting clOrdId")
	}

	var partyId quickfix.FIXString
	err = msg.GetField(tag.PartyID, &partyId)
	if err != nil {
		logs.Log.Err(err).Msg("Error getting partyId")
	}

	user, e := a.UserRepository.FindById(context.TODO(), userId)
	if e != nil {
		return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
	}

	type MassCancel struct {
		symbol  string
		orderId string
	}
	orderIds := []MassCancel{}
	if symbol != "all" {
		orders, _ := a.OrderRepository.GetOpenOrdersByInstrument(symbol, "limit", userId)
		for _, order := range orders {
			orderIds = append(orderIds, MassCancel{
				symbol:  order.InstrumentName,
				orderId: order.OrderId.Hex(),
			})
		}
	} else {
		orders, _ := a.OrderRepository.Find(bson.M{"userId": userId, "status": "open", "type": "limit"}, nil, 0, -1)
		for _, order := range orders {
			orderIds = append(orderIds, MassCancel{
				symbol:  order.Underlying + "-" + order.ExpiryDate + "-" + strconv.FormatFloat(order.StrikePrice, 'f', 0, 64) + "-" + string(order.Contracts)[0:1],
				orderId: order.ID.Hex(),
			})
		}
	}

	for _, orderId := range orderIds {
		response, reason, r := a.DeribitService.DeribitRequest(context.TODO(), user.ID.Hex(), _deribitModel.DeribitRequest{
			ID:             orderId.orderId,
			ClOrdID:        clOrdID,
			ClientId:       partyId.String(),
			Side:           _utilitiesType.CANCEL,
			InstrumentName: orderId.symbol,
			Type:           _utilitiesType.LIMIT,
		})

		requestType := enum.MassCancelRequestType_CANCEL_ORDERS_FOR_A_SECURITY
		cancelResponse := enum.MassCancelResponse_CANCEL_ORDERS_FOR_AN_UNDERLYING_SECURITY

		if r != nil {
			cancelResponse = enum.MassCancelResponse_CANCEL_REQUEST_REJECTED
			if reason != nil {
				logs.Log.Err(r).Msg(fmt.Sprintf("Failed to send cancel request for %v, %v", orderId, reason.String()))
			}
			logs.Log.Err(r).Msg(fmt.Sprintf("Failed to send cancel request for %v", orderId))
			continue
		}

		if symbol == "" {
			requestType = enum.MassCancelRequestType_CANCEL_ALL_ORDERS
			cancelResponse = enum.MassCancelResponse_CANCEL_ALL_ORDERS
		}

		msg := ordermasscancelreport.New(
			field.NewOrderID(response.ID),
			field.NewMassCancelRequestType(requestType),
			field.NewMassCancelResponse(cancelResponse),
		)

		msg.SetOrderID(response.ID)
		msg.SetClOrdID(response.ClOrdID)
		if symbol != "" {
			msg.SetSymbol(symbol)
		}
		err := quickfix.SendToTarget(msg, *userSession[response.UserId])
		if err != nil {
			fmt.Print(err.Error())
		}
	}

	return nil
}

// 37 Order ID
// 448 Party ID
func (a *Application) onOrderCancelRequest(msg ordercancelrequest.OrderCancelRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Println("onOrderCancelRequest")
	userId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			userId = i
		}
	}

	// user, e := a.UserRepository.FindById(context.TODO(), userId)
	// if e != nil {
	// 	return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
	// }

	orderId, err := msg.GetOrderID()
	if err != nil {
		return err
	}

	// clOrdID, err := msg.GetClOrdID()
	// if err != nil {
	// 	return err
	// }

	// symbol, err := msg.GetSymbol()
	// if err != nil {
	// 	return err
	// }
	var partyId quickfix.FIXString
	msg.GetField(tag.PartyID, &partyId)

	// TODO: party id
	// Call cancel service
	// Call service
	_, r := a.DeribitService.DeribitParseCancel(context.Background(), userId, _deribitModel.DeribitCancelRequest{
		Id:      orderId,
		ClOrdID: partyId.String(),
	})

	// _, reason, r := a.DeribitService.DeribitRequest(context.TODO(), user.ID.Hex(), _deribitModel.DeribitRequest{
	// 	ID:             orderId,
	// 	ClOrdID:        clOrdID,
	// 	ClientId:       partyId.String(),
	// 	Side:           _utilitiesType.CANCEL,
	// 	InstrumentName: symbol,
	// 	Type:           _utilitiesType.LIMIT,
	// })

	if r != nil {
		fmt.Println(r)
		logs.Log.Err(r).Msg("Failed to send cancel request")
		return quickfix.NewMessageRejectError("Failed to send cancel request", 1, nil)
	}

	// if reason != nil {
	// 	a.sendOrderCancelReject(
	// 		field.NewOrderID(orderId),
	// 		field.NewClOrdID(clOrdID),
	// 		field.NewOrigClOrdID(clOrdID),
	// 		field.NewOrdStatus(enum.OrdStatus_REJECTED),
	// 		field.NewCxlRejResponseTo(enum.CxlRejResponseTo_ORDER_CANCEL_REQUEST),
	// 		field.NewText(reason.String()),
	// 		sessionID)
	// 	logs.Log.Err(r).Msg(fmt.Sprintf("Failed to send cancel request, %v", reason.String()))
	// 	return nil
	// }

	return nil
}

func (a *Application) sendOrderCancelReject(
	orderId field.OrderIDField,
	clordid field.ClOrdIDField,
	origclordid field.OrigClOrdIDField,
	ordstatus field.OrdStatusField,
	cxlrejresponseTo field.CxlRejResponseToField,
	reason field.TextField,
	sessionID quickfix.SessionID) {
	msg := ordercancelreject.New(
		orderId,
		clordid,
		origclordid,
		ordstatus,
		cxlrejresponseTo,
	)
	msg.SetText(reason.String())
	err := quickfix.SendToTarget(msg, sessionID)
	if err != nil {
		logs.Log.Err(err).Msg("Failed sending order cancel reject")
	}
}

// MsgType V
func (a *Application) onMarketDataRequest(msg marketdatarequest.MarketDataRequest, sessionID quickfix.SessionID) (err quickfix.MessageRejectError) {
	subs, _ := msg.GetSubscriptionRequestType()

	// 0 = BID, 1 = ASK, 2 = TRADE
	mdEntryTypes := marketdatarequest.NewNoMDEntryTypesRepeatingGroup()
	err = msg.GetGroup(mdEntryTypes)
	if err != nil {
		logs.Log.Err(err).Msg("Error getting group")
	}

	//entries contain the type of market data requested (bid, ask, trade)
	entries := make([]string, mdEntryTypes.Len())
	for j := 0; j < mdEntryTypes.Len(); j++ {
		entry, _ := mdEntryTypes.Get(j).GetMDEntryType()
		entries = append(entries, string(entry))
	}

	noRelatedsym, _ := msg.GetNoRelatedSym()

	// loop based on symbol requested
	for i := 0; i < noRelatedsym.Len(); i++ {
		response := []MarketDataResponse{}
		sym, _ := noRelatedsym.Get(i).GetSymbol()

		_, errParse := utils.ParseInstruments(sym, false)
		if errParse != nil {
			logs.Log.Err(errParse).Msg("Error parsing instrument")
			return
		}

		// Handle Subscriber
		if subs == enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES {
			VSubscribe(sym, sessionID)
		}

		// Split Instrument Name
		splits := strings.Split(sym, "-")
		price, _ := strconv.ParseFloat(splits[2], 64)

		// Get Order Book (bids and asks)
		_order := _orderbookType.GetOrderBook{
			InstrumentName: sym,
			Underlying:     splits[0],
			ExpiryDate:     splits[1],
			StrikePrice:    price,
		}
		_orderbook := a.OrderRepository.GetOrderBook(_order)

		// 0 means BID / BUY
		if utils.ArrContains(entries, "0") {
			// asks := a.OrderRepository.GetMarketData(sym, "buy")
			// Loop Order Book Bids
			for _, bid := range _orderbook.Bids {
				response = append(response, MarketDataResponse{
					Price:          bid.Price,
					Amount:         bid.Amount,
					Side:           "buy",
					EntryType:      enum.MDEntryType_BID,
					InstrumentName: sym,
					Type:           "bid",
				})
			}
		}

		// 1 means ASK / sell
		if utils.ArrContains(entries, "1") {
			for _, ask := range _orderbook.Asks {
				response = append(response, MarketDataResponse{
					Price:          ask.Price,
					Amount:         ask.Amount,
					Side:           "sell",
					EntryType:      enum.MDEntryType_OFFER,
					InstrumentName: sym,
					Type:           "ask",
				})
			}

		}

		// if utils.ArrContains(entries, "2") {
		// 	splits := strings.Split(sym, "-")
		// 	price, _ := strconv.ParseFloat(splits[2], 64)
		// 	trades, _ := a.TradeRepository.Find(bson.M{
		// 		"underlying":  splits[0],
		// 		"expiryDate":  splits[1],
		// 		"strikePrice": price,
		// 		"status":      "success",
		// 	}, nil, 0, -1)
		// 	for _, trade := range trades {
		// 		conversion, _ := utils.ConvertToFloat(trade.Amount)
		// 		response = append(response, MarketDataResponse{
		// 			Price:  trade.Price,
		// 			Amount: conversion,
		// 			Side:   trade.Side,
		// 			InstrumentName: trade.Underlying + "-" + trade.ExpiryDate + "-" + strconv.FormatFloat(trade.StrikePrice, 'f', 0, 64) +
		// 				"-" + string(trade.Contracts)[0:1],
		// 			Date:    trade.CreatedAt.String(),
		// 			Type:    "TRADE",
		// 			MakerID: trade.Maker.OrderID.Hex(),
		// 			TakerID: trade.Taker.OrderID.Hex(),
		// 			Status:  string(trade.Status),
		// 		})
		// 	}
		// }

		if len(response) == 0 {
			continue
		}
		snap := marketdatasnapshotfullrefresh.New()
		snap.SetSymbol(response[0].InstrumentName)
		reqId, _ := msg.GetMDReqID()
		snap.SetMDReqID(reqId)
		grp := marketdatasnapshotfullrefresh.NewNoMDEntriesRepeatingGroup()
		response = mapMarketDataResponse(response)

		bytes, _ := json.Marshal(response)
		a.redis.Set("MARKETDATA-"+response[0].InstrumentName, string(bytes))
		for _, res := range response {
			row := grp.Add()
			row.SetMDEntryType(res.EntryType)                       // 269
			row.SetMDEntrySize(decimal.NewFromFloat(res.Amount), 2) // 271
			row.SetMDEntryPx(decimal.NewFromFloat(res.Price), 2)    // 270
			row.SetMDEntryDate(res.Date)
			row.SetOrderID(res.MakerID)
		}
		snap.SetNoMDEntries(grp)
		error := quickfix.SendToTarget(snap, sessionID)
		if error != nil {
			logs.Log.Err(error).Msg("Error sending market data")
		}
	}

	return
}

// Hook when ENGINE_SAVED topic received
func (a *Application) OnEngineSavedReceived(data types.EngineResponse) {
	// Broadcast Best Bid and Best Ask (market price)
	takerOrder := data.Matches.TakerOrder
	instrumentName := takerOrder.Underlying + "-" + takerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", takerOrder.StrikePrice) + "-" + string(takerOrder.Contracts[0])
	a.BroadcastQuoteStatusReport(instrumentName)

	// Broadcast NEW Security List / Instrument Name
	a.BroadcastSecurityList(instrumentName)
}

// Hook when Engine topic received
func (a *Application) OnTradeHappens(data types.EngineResponse) {
	// Handle Ticks
	// Broadcast last price and volume based on instrument name
	BroadcastTrades(data.Matches.Trades)

	// Broadcast LastPx and LastQty
	// TradeCaptureReport - tradecapturereport.go
	a.BroadcastTradeCaptureReport(data.Matches.Trades)

	takerOrder := data.Matches.TakerOrder
	instrumentName := takerOrder.Underlying + "-" + takerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", takerOrder.StrikePrice) + "-" + string(takerOrder.Contracts[0])

	// Broadcast order book
	a.BroadcastOrderBook(instrumentName)
}

// Broadcast whole order book
// Whoever submit GetMarketDataRequest will receive this broadcast
func (a *Application) BroadcastOrderBook(instrument string) {

	response := []MarketDataResponse{}

	splits := strings.Split(instrument, "-")
	price, _ := strconv.ParseFloat(splits[2], 64)

	_order := _orderbookType.GetOrderBook{
		InstrumentName: instrument,
		Underlying:     splits[0],
		ExpiryDate:     splits[1],
		StrikePrice:    price,
	}
	_orderbook := a.OrderRepository.GetOrderBook(_order)

	// Loop Order Book Bids
	for _, bid := range _orderbook.Bids {
		response = append(response, MarketDataResponse{
			Price:          bid.Price,
			Amount:         bid.Amount,
			Side:           "buy",
			EntryType:      enum.MDEntryType_BID,
			InstrumentName: instrument,
			Type:           "bid",
		})
	}

	// 1 means ASK / sell
	for _, ask := range _orderbook.Asks {
		response = append(response, MarketDataResponse{
			Price:          ask.Price,
			Amount:         ask.Amount,
			Side:           "sell",
			EntryType:      enum.MDEntryType_OFFER,
			InstrumentName: instrument,
			Type:           "ask",
		})
	}

	if len(response) == 0 {
		fmt.Println("debug orderbook nil response")
		return
	}

	snap := marketdatasnapshotfullrefresh.New()
	snap.SetSymbol(instrument)
	snap.SetMDReqID("notification")
	grp := marketdatasnapshotfullrefresh.NewNoMDEntriesRepeatingGroup()
	response = mapMarketDataResponse(response)
	for _, res := range response {
		row := grp.Add()
		row.SetMDEntryType(res.EntryType)                       // 269
		row.SetMDEntrySize(decimal.NewFromFloat(res.Amount), 2) // 271
		row.SetMDEntryPx(decimal.NewFromFloat(res.Price), 2)    // 270
		row.SetMDEntryDate(res.Date)
		row.SetOrderID(res.MakerID)
	}
	snap.SetNoMDEntries(grp)

	// Broadcast to all subscribers
	for sessionID, status := range subsManager.VSubscriptions[instrument] {
		if status {
			err := quickfix.SendToTarget(snap, sessionID)
			if err != nil {
				logs.Log.Err(err).Msg("Error sending quote capture request update")
			}
		}
	}
}

// Broadcast last price and volume based on instrument name
// Whoever submit GetMarketDataRequest will receive this broadcast
func BroadcastTrades(trades []*types.Trade) {

	// check whether trades is not empty array
	if len(trades) == 0 {
		return
	}

	// Check whether we have subscribers
	if len(vMessageSubs) == 0 {
		fmt.Println("debug No subscribers")
		return
	}

	fmt.Println("debug got subscribers")

	// Create a new MarketDataIncrementalRefresh message
	msg := marketdataincrementalrefresh.New()

	msg.SetMDReqID("tick") //tag 262
	grp := marketdataincrementalrefresh.NewNoMDEntriesRepeatingGroup()
	for _, trade := range trades {
		row := grp.Add()
		symbol := trade.Underlying + "-" + trade.ExpiryDate + "-" + fmt.Sprintf("%.0f", trade.StrikePrice) + "-" + string(trade.Contracts[0])
		fmt.Println("debug symbol", symbol)

		row.SetMDUpdateAction(enum.MDUpdateAction_NEW)

		// Symbol
		row.SetSymbol(symbol)
		// Price. Tag 270
		row.SetMDEntryPx(decimal.NewFromFloat(trade.Price), 2)
		// Amount. Tag 271
		amt, _ := utils.ConvertToFloat(trade.Amount)
		row.SetMDEntrySize(decimal.NewFromFloat(amt), 2)
		// Type. Tag 269
		row.SetMDEntryType(enum.MDEntryType_TRADE)
	}
	msg.SetNoMDEntries(grp)

	// Broadcast to all subscribers
	for _, sess := range vMessageSubs {
		fmt.Println("debug SendToTarget")
		err := quickfix.SendToTarget(msg, sess.sessiondID)
		if err != nil {
			logs.Log.Err(err).Msg("Error sending tick update")
		}
	}
}

func (a *Application) GetTrade(filter bson.M) (trades []*types.Trade) {
	trades, _ = a.TradeRepository.Find(filter, nil, 0, -1)
	return trades
}

func OnMarketDataUpdate(instrument string, book _orderbookType.BookData) {
	// FIX ME bug found here
	return
	for _, subs := range vMessageSubs {
		response := []MarketDataResponse{}
		if subs.InstrumentName != instrument {
			continue
		}

		if subs.Bid {
			for _, bid := range book.Bids {
				response = append(response, MarketDataResponse{
					Price:          bid[1].(float64),
					Amount:         bid[2].(float64),
					InstrumentName: instrument,
					Type:           "BID",
					UpdateType:     bid[0].(string),
					Side:           "BUY",
				})
			}
		}
		if subs.Ask {
			for _, ask := range book.Asks {
				response = append(response, MarketDataResponse{
					Price:          ask[1].(float64),
					Amount:         ask[2].(float64),
					InstrumentName: instrument,
					Type:           "ASK",
					Side:           "SELL",
					UpdateType:     ask[0].(string),
				})
			}
		}
		if subs.Trade {
			splits := strings.Split(instrument, "-")
			price, _ := strconv.ParseFloat(splits[2], 64)
			trades := newApplication(nil, nil).GetTrade(bson.M{
				"underlying":  splits[0],
				"expiryDate":  splits[1],
				"strikePrice": price,
				"status":      "success",
			})

			for _, trade := range trades {
				conversion, _ := utils.ConvertToFloat(trade.Amount)
				response = append(response, MarketDataResponse{
					Price:          trade.Price,
					Amount:         conversion,
					InstrumentName: instrument,
					Type:           "TRADE",
					UpdateType:     "change",
					Side:           trade.Side,
				})
			}
		}

		msg := marketdataincrementalrefresh.New()
		msg.SetMDReqID("orderbook") //tag 262
		grp := marketdataincrementalrefresh.NewNoMDEntriesRepeatingGroup()
		for _, res := range response {
			update := enum.MDUpdateAction_NEW
			if res.UpdateType == "delete" {
				update = enum.MDUpdateAction_DELETE
			} else if res.UpdateType == "change" {
				update = enum.MDUpdateAction_CHANGE
			}
			row := grp.Add()
			row.SetSymbol(res.InstrumentName)
			row.SetMDUpdateAction(update)
			row.SetMDEntryType(enum.MDEntryType(res.Side))
			row.SetMDEntrySize(decimal.NewFromFloat(res.Amount), 2)
			row.SetMDEntryPx(decimal.NewFromFloat(res.Price), 2)
			row.SetMDEntryDate(res.Date)
		}
		msg.SetNoMDEntries(grp)
		if err := quickfix.SendToTarget(msg, subs.sessiondID); err != nil {
			logs.Log.Err(err).Msg("Error sending market data")
		}
	}
}

func mapMarketDataResponse(res []MarketDataResponse) []MarketDataResponse {
	result := []MarketDataResponse{}

	for i, r := range res {
		if len(result) == 0 {
			result = append(result, r)
			continue
		}
		exist := isInstrumentExists(result, r)
		if !exist {
			result = append(result, r)
			if i == len(res)-1 {
				return result
			}
		}
	}
	after := sumAmount(result, res)
	return after
}

func isInstrumentExists(data []MarketDataResponse, marketData MarketDataResponse) bool {
	for _, d := range data {
		if d.InstrumentName == marketData.InstrumentName && d.Price == marketData.Price && d.Side == marketData.Side {
			return true
		}
	}
	return false
}

func sumAmount(data []MarketDataResponse, og []MarketDataResponse) (res []MarketDataResponse) {
	for _, r := range data {
		amount := float64(0)
		for _, rr := range og {
			if r.InstrumentName == rr.InstrumentName && r.Price == rr.Price && r.Side == rr.Side {
				amount += rr.Amount
			}
		}
		r.Amount = amount
		res = append(res, r)
	}
	return res
}

func OnMatchingOrder(data types.EngineResponse) {
	return

	// No matches, then nothing to do
	if data.Matches == nil {
		return
	}

	// No trades, then nothing to do
	if data.Matches.Trades == nil {
		return
	}

	// This is the update, then only cares about maker
	for _, trd := range data.Matches.MakerOrders {
		if userSession == nil {
			return
		}

		// If the maker user ID doesnt have the session in the FIX
		// then do nothing
		sessionID := userSession[trd.Order.UserID.Hex()]
		if sessionID == nil {
			return
		}

		// order := data.Matches.TakerOrder
		order := trd.Order
		conversion, _ := utils.ConvertToFloat(order.FilledAmount)

		// FIX Side
		fixSide := enum.Side_BUY
		if order.Side == _utilitiesType.BUY {
			fixSide = enum.Side_SELL
		}

		// Exec type https://www.onixs.biz/fix-dictionary/4.4/tagnum_150.html
		// FIX Order Status
		fixStatus := enum.OrdStatus_NEW
		fixExecType := enum.ExecType_NEW
		if order.Status == _utilitiesType.FILLED {
			fixStatus = enum.OrdStatus_FILLED
			fixExecType = enum.ExecType_FILL
		} else if order.Status == _utilitiesType.PARTIALLY_FILLED {
			fixStatus = enum.OrdStatus_PARTIALLY_FILLED
			fixExecType = enum.ExecType_TRADE
		} else if order.Status == _utilitiesType.CANCELLED {
			fixStatus = enum.OrdStatus_CANCELED
			fixExecType = enum.ExecType_TRADE
		}

		msg := executionreport.New(
			field.NewOrderID(trd.ID.Hex()),
			field.NewExecID(order.ClOrdID),
			field.NewExecType(fixExecType),
			field.NewOrdStatus(fixStatus),
			field.NewSide(fixSide),
			field.NewLeavesQty(decimal.NewFromFloat(trd.Amount), 2),
			field.NewCumQty(decimal.NewFromFloat(conversion), 2),
			field.NewAvgPx(decimal.NewFromFloat(trd.Price), 2),
		)

		msg.SetClOrdID(trd.ClOrdID)
		msg.SetLastPx(decimal.NewFromFloat(trd.Price), 2)
		msg.SetLastQty(decimal.NewFromFloat(trd.Amount), 2)

		if err := quickfix.SendToTarget(msg, *sessionID); err != nil {
			logs.Log.Err(err).Msg("Error notifying FIX session order")
		}
	}

}

func OnOrderboookUpdate(symbol string, data map[string]interface{}) {

	bids := data["bids"].([]Order)
	asks := data["asks"].([]Order)

	msg := marketdatasnapshotfullrefresh.New()

	for _, bid := range bids {
		mdBid := field.NewNoMDEntryTypes(1)
		underlying := field.NewUnderlyingSymbol(bid.Underlying)
		strike := field.NewStrikePrice(decimal.NewFromFloat(bid.StrikePrice), 10)
		side := field.NewSide(enum.Side(bid.Side))
		amount := field.NewQuantity(bid.Amount, 10)
		filled := field.NewFillQty(bid.FilledAmount, 10)
		exp := field.NewExDate(bid.ExpiryDate)
		price := field.NewPrice(bid.Amount, 10)
		status := field.NewStatusText(bid.Status)

		grp := marketdatasnapshotfullrefresh.NewNoMDEntriesRepeatingGroup()
		grp.Add().Set(mdBid)
		grp.Add().Set(underlying)
		grp.Add().Set(strike)
		grp.Add().Set(side)
		grp.Add().Set(amount)
		grp.Add().Set(filled)
		grp.Add().Set(exp)
		grp.Add().Set(price)
		grp.Add().Set(status)
		msg.SetNoMDEntries(grp)
	}

	for _, ask := range asks {
		mdAsk := field.NewNoMDEntryTypes(2)
		underlying := field.NewUnderlyingSymbol(ask.Underlying)
		strike := field.NewStrikePrice(decimal.NewFromFloat(ask.StrikePrice), 10)
		side := field.NewSide(enum.Side(ask.Side))
		amount := field.NewQuantity(ask.Amount, 10)
		filled := field.NewFillQty(ask.FilledAmount, 10)
		exp := field.NewExDate(ask.ExpiryDate)
		price := field.NewPrice(ask.Price, 10)
		status := field.NewStatusText(ask.Status)

		grp := marketdatasnapshotfullrefresh.NewNoMDEntriesRepeatingGroup()
		grp.Add().Set(mdAsk)
		grp.Add().Set(underlying)
		grp.Add().Set(strike)
		grp.Add().Set(side)
		grp.Add().Set(amount)
		grp.Add().Set(filled)
		grp.Add().Set(exp)
		grp.Add().Set(price)
		grp.Add().Set(status)
		msg.SetNoMDEntries(grp)
	}

	for _, sess := range vMessageSubs {
		err := quickfix.SendToTarget(msg, sess.sessiondID)
		if err != nil {
			logs.Log.Err(err).Msg("Error sending orderbook update")
		}
	}
}

func (a *Application) acceptOrder(order Order) {
	a.updateOrder(order, enum.OrdStatus_NEW)
}

func (a *Application) fillOrder(order Order) {
	status := enum.OrdStatus_FILLED
	// if !order.IsClosed() {
	// 	status = enum.OrdStatus_PARTIALLY_FILLED
	// }
	a.updateOrder(order, status)
}

func (a *Application) cancelOrder(order Order) {
	a.updateOrder(order, enum.OrdStatus_CANCELED)
}

func (a *Application) genExecID() string {
	a.execID++
	return strconv.Itoa(a.execID)
}

// updateOrder sends an execution report to the client for the given order (fill/cancel)
func (a *Application) updateOrder(order Order, status enum.OrdStatus) {
	execReport := executionreport.New(
		field.NewOrderID(order.ID),
		field.NewExecID(a.genExecID()),
		field.NewExecType(enum.ExecType(status)),
		field.NewOrdStatus(status),
		field.NewSide(order.Side),
		field.NewLeavesQty(order.Amount.Sub(order.FilledAmount), 2),
		field.NewCumQty(order.FilledAmount, 2),
		field.NewAvgPx(order.Price, 2),
	)
	execReport.SetString(quickfix.Tag(448), order.ClientID)
	execReport.SetOrderQty(order.Amount, 2)
	execReport.SetClOrdID(order.ID)
	execReport.SetString(quickfix.Tag(448), order.ClientID)

	switch status {
	case enum.OrdStatus_FILLED, enum.OrdStatus_PARTIALLY_FILLED:
		execReport.SetLastQty(order.LastExecutedQuantity, 2)
		execReport.SetLastPx(order.LastExecutedPrice, 2)
	}

	execReport.Header.SetTargetCompID(order.SenderCompID)
	execReport.Header.SetSenderCompID(order.TargetCompID)

	sendErr := quickfix.Send(execReport)
	if sendErr != nil {
		logs.Log.Err(sendErr).Msg("Error sending execution report")
	}

}

// 35 Execution Report
// There are 2 things here:
// 1. Construct execution report for the taker order (the one that is being executed)
// 2. If the order is partially filled/filled (based on the trades), then add LastQty and LastPx
// For point 2, we need several things:
// - 1363	FillExecID, 1364	FillPx, 1365	FillQty
func (a *Application) OrderConfirmation(data types.EngineResponse) {
	fmt.Println("OrderConfirmation")
	userId := data.Matches.TakerOrder.UserID.Hex()
	takerOrder := data.Matches.TakerOrder
	symbol := takerOrder.Underlying + "-" + takerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", takerOrder.StrikePrice) + "-" + string(takerOrder.Contracts[0])

	// Point 1, execution report for the taker order
	if userSession == nil {
		if userSession[userId] == nil {
			return
		}
		return
	}
	sessionId := userSession[userId]
	if sessionId == nil {
		return
	}

	exec := 0
	switch takerOrder.Status {
	case "FILLED":
		exec = 2
		break
	case "PARTIALLY FILLED":
		exec = 1
		break
	case "ORDER_CANCELLED":
		exec = 4
	}

	// FIX Side
	fixSide := enum.Side_BUY
	if takerOrder.Side == _utilitiesType.BUY {
		fixSide = enum.Side_SELL
	}

	// FIX Order Status
	fixStatus := enum.OrdStatus_NEW
	fixExecType := enum.ExecType_NEW
	if takerOrder.Status == _utilitiesType.FILLED {
		fixStatus = enum.OrdStatus_FILLED
		fixExecType = enum.ExecType_TRADE
	} else if takerOrder.Status == _utilitiesType.PARTIALLY_FILLED {
		fixStatus = enum.OrdStatus_PARTIALLY_FILLED
		fixExecType = enum.ExecType_TRADE
	} else if takerOrder.Status == _utilitiesType.CANCELLED {
		fixStatus = enum.OrdStatus_CANCELED
		fixExecType = enum.ExecType_CANCELED
	}

	conversion, _ := utils.ConvertToFloat(takerOrder.FilledAmount)

	msg := executionreport.New(
		field.NewOrderID(takerOrder.ID.Hex()), // 37
		field.NewExecID(strconv.Itoa(exec)),   // 17
		field.NewExecType(fixExecType),        // 150
		field.NewOrdStatus(fixStatus),         // 39
		field.NewSide(fixSide),                // 54
		field.NewLeavesQty(decimal.NewFromFloat(takerOrder.Amount).Sub(decimal.NewFromFloat(conversion)), 2), // Order quantity open for further execution (LeavesQty = OrderQty - CumQty) 151
		field.NewCumQty(decimal.NewFromFloat(conversion), 2),                                                 // Executed Qty 14
		field.NewAvgPx(decimal.NewFromFloat(takerOrder.Price), 2),                                            // 6 TODO: FIX ME
	)

	// ClOrderID -- Order MT ID in MT5
	msg.SetClOrdID(takerOrder.ClOrdID) // 11

	// Setting price to the FIX message
	msg.SetPrice(decimal.NewFromFloat(takerOrder.Price), 2) // 44

	// Instrument Name. Tag 55
	msg.SetSymbol(symbol) // 55

	// Setting Party ID == Order . Client ID -- Login in MT5. Tag 448
	msg.Set(field.NewPartyID(takerOrder.ClientID))

	// Setting Order Type (Market or Limit) -- Tag 40
	if takerOrder.Type == _utilitiesType.MARKET {
		msg.SetOrdType(enum.OrdType_MARKET)
	} else {
		msg.SetOrdType(enum.OrdType_LIMIT)
	}

	// Point 2, check if there's trades
	if conversion == 0 {
		fmt.Println("debug no trades")
		msg.SetLastQty(decimal.NewFromFloat(0), 2) // 32
		msg.SetLastPx(decimal.NewFromFloat(0), 2)  // 31
	} else {
		fmt.Println("debug there are trades")
		msg.SetLastQty(decimal.NewFromFloat(conversion), 2)      // 32
		msg.SetLastPx(decimal.NewFromFloat(takerOrder.Price), 2) // 31
	}

	err := quickfix.SendToTarget(msg, *sessionId)
	if err != nil {
		fmt.Print(err.Error())
	}

}

func (a *Application) MakerConfirmation(data types.EngineResponse) {
	// Check if there's any trades
	if len(data.Matches.Trades) == 0 {
		return
	}

	fmt.Println("debug MakerConfirmation")

	for _, trade := range data.Matches.Trades {
		// Only carse about maker

		userId := trade.Maker.UserID.Hex()
		// CHeck if the user has FIX session
		if userSession[userId] == nil {
			continue
		}
		sessionId := userSession[userId]

		makerOrderID := trade.Maker.OrderID.Hex()
		// Find order based on the engine response
		// Do not query into the database - since it may need sometime for DB to reflect
		var makerOrder *_orderbookType.Order

		for _, order := range data.Matches.MakerOrders {
			if order.UserID.Hex() == userId && order.ID.Hex() == makerOrderID {
				makerOrder = order
				break
			}
		}

		if makerOrder == nil {
			fmt.Println("debug makerOrder nil")
			continue
		}

		//makerOrder, _ := newApplication(nil).OrderRepository.GetOrderById(makerOrderID);
		symbol := makerOrder.Underlying + "-" + makerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", makerOrder.StrikePrice) + "-" + string(makerOrder.Contracts[0])

		execID := "KPR"
		execType := enum.ExecType_TRADE
		ordStatus := enum.OrdStatus_FILLED
		side := enum.Side_BUY
		conversion, _ := utils.ConvertToFloat(makerOrder.FilledAmount)

		if makerOrder.Status == _utilitiesType.PARTIALLY_FILLED {
			ordStatus = enum.OrdStatus_PARTIALLY_FILLED
		}
		if makerOrder.Side == _utilitiesType.BUY {
			side = enum.Side_SELL
		}

		trade_amount_conversion, _ := utils.ConvertToFloat(trade.Amount)

		leavesQty := decimal.NewFromFloat(makerOrder.Amount).Sub(decimal.NewFromFloat(conversion))
		cumQty := decimal.NewFromFloat(trade_amount_conversion)
		avgPx := decimal.NewFromFloat(0)

		fmt.Println("debug makerOrder.ID.Hex()", makerOrder.ID.Hex())
		fmt.Println("debug makerOrder.ID", makerOrder.ID)

		msg := executionreport.New(
			field.NewOrderID(makerOrder.ID.Hex()), // 37
			field.NewExecID(execID),               // 17
			field.NewExecType(execType),           // 150
			field.NewOrdStatus(ordStatus),         // 39
			field.NewSide(side),                   // 54
			field.NewLeavesQty(leavesQty, 2),      // Order quantity open for further execution (LeavesQty = OrderQty - CumQty) 151
			field.NewCumQty(cumQty, 2),            // Executed Qty 14
			field.NewAvgPx(avgPx, 2),              // 6 TODO: FIX ME
		)

		// ClOrderID -- Order MT ID in MT5
		msg.SetClOrdID(makerOrder.ClOrdID) // 11

		// Setting price to the FIX message
		msg.SetPrice(decimal.NewFromFloat(trade.Price), 2) // 44

		// Instrument Name. Tag 55
		msg.SetSymbol(symbol) // 55

		// Setting Party ID == Order . Client ID -- Login in MT5. Tag 448
		msg.Set(field.NewPartyID(makerOrder.ClientID))

		// Setting Order Type (Market or Limit) -- Tag 40
		if makerOrder.Type == _utilitiesType.MARKET {
			msg.SetOrdType(enum.OrdType_MARKET)
		} else {
			msg.SetOrdType(enum.OrdType_LIMIT)
		}

		// Convert trade amount from string to float
		conversion, _ = utils.ConvertToFloat(trade.Amount)
		msg.SetLastQty(decimal.NewFromFloat(conversion), 2) // 32
		msg.SetLastPx(decimal.NewFromFloat(trade.Price), 2) // 31

		err := quickfix.SendToTarget(msg, *sessionId)
		if err != nil {
			fmt.Print(err.Error())
		}
	}

}

func removeXMessageSubscriber(array []XMessageSubscriber, element XMessageSubscriber) []XMessageSubscriber {
	for i, v := range array {
		if v == element {
			return append(array[:i], array[i+1:]...)
		}
	}
	return array
}

func addVMessagesSubscriber(array []VMessageSubscriber, sessionID quickfix.SessionID, bid bool, ask bool, trade bool, symbolsEntry marketdatarequest.NoRelatedSymRepeatingGroup) []VMessageSubscriber {
	exist := false
	for i := 0; i < symbolsEntry.Len(); i++ {
		for _, v := range array {
			if v.sessiondID == sessionID {
				exist = true
			}
		}
		if !exist {
			instrument, _ := symbolsEntry.Get(i).GetSymbol()
			array = append(array, VMessageSubscriber{
				InstrumentName: instrument,
				sessiondID:     sessionID,
				Bid:            bid,
				Ask:            ask,
				Trade:          trade,
			})
		}
	}
	return array
}

func removeVMessageSubscriber(array []VMessageSubscriber, element VMessageSubscriber) []VMessageSubscriber {
	for i, v := range array {
		if v == element {
			return append(array[:i], array[i+1:]...)
		}
	}
	return array
}

const (
	usage = "ordermatch"
	short = "Start an order matching (FIX acceptor) service"
	long  = "Start an order matching (FIX acceptor) service."
)

func InitApp(deribit _deribitSvc.IDeribitService, wsSvc _wsSvc.IwsOrderbookService) *Application {
	return newApplication(deribit, wsSvc)
}

func Execute(app *Application) error {
	cfgFileName := "ordermatch.cfg"
	templateCfg := "ordermatch_template.cfg"
	_, b, _, _ := runtime.Caller(0)

	input, _ := ioutil.ReadFile(path.Join(b, "../", "config", templateCfg))

	config := strings.ReplaceAll(string(input), "$DATA_DICTIONARY_PATH", os.Getenv("DATA_DICTIONARY_PATH"))

	ioutil.WriteFile(path.Join(b, "../", "config", cfgFileName), []byte(config), 0644)

	cfg, err := os.Open(path.Join(b, "../", "config", cfgFileName))
	if err != nil {
		return fmt.Errorf("error opening %v, %v", cfgFileName, err)
	}
	defer cfg.Close()
	stringData, readErr := io.ReadAll(cfg)
	if readErr != nil {
		return fmt.Errorf("error reading cfg: %s,", readErr)
	}

	appSettings, err := quickfix.ParseSettings(bytes.NewReader(stringData))
	if err != nil {
		return fmt.Errorf("error reading cfg: %s,", err)
	}

	logger := log.NewFancyLog()
	// app := newApplication(deribit)
	acceptor, err := quickfix.NewAcceptor(app, quickfix.NewMemoryStoreFactory(), appSettings, logger)
	if err != nil {
		return fmt.Errorf("unable to create acceptor: %s", err)
	}

	err = acceptor.Start()
	if err != nil {
		return fmt.Errorf("unable to start FIX acceptor: %s", err)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interrupt
		acceptor.Stop()
		os.Exit(0)
	}()

	return nil
}
