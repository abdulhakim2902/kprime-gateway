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
	"gateway/internal/engine/types"
	"gateway/internal/repositories"
	"gateway/pkg/utils"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	_mongo "gateway/pkg/mongo"

	"git.devucc.name/dependencies/utilities/commons/log"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/executionreport"
	"github.com/quickfixgo/fix44/marketdatarequest"
	"github.com/quickfixgo/fix44/marketdatasnapshotfullrefresh"
	"github.com/quickfixgo/fix44/newordersingle"
	"github.com/quickfixgo/fix44/ordercancelreplacerequest"
	"github.com/quickfixgo/fix44/ordercancelrequest"
	"github.com/quickfixgo/fix44/securitylist"
	"github.com/quickfixgo/fix44/securitylistrequest"
	"github.com/quickfixgo/tag"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"

	_producer "gateway/pkg/kafka/producer"

	"github.com/quickfixgo/quickfix"
)

type XMessageSubscriber struct {
	sessiondID quickfix.SessionID
	secReq     string
}

type VMessageSubscriber struct {
	sessiondID quickfix.SessionID
}

var userSession map[string]*quickfix.SessionID
var vMessageSubs []VMessageSubscriber
var xMessagesSubs []XMessageSubscriber

type Order struct {
	ID                   string          `json:"id" bson:"_id"`
	ClientOrderId        string          `json:"clOrdId" bson:"clOrdId"`
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
	InstrumentName string  `json:"instrumentName"`
	Side           string  `json:"side"`
	Contract       string  `json:"contract"`
	Price          float64 `json:"price"`
	Amount         float64 `json:"amount"`
	Date           string  `json:"date"`
	Type           string  `json:"type"`
	MakerID        string  `json:"makerId"`
	TakerID        string  `json:"takerId"`
	Status         string  `json:"status"`
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
}

type KafkaOrder struct {
	ID             string  `json:"id"`
	ClOrdID        string  `json:"clOrdID,omitempty"`
	UserID         string  `json:"userId,omitempty"`
	ClientID       string  `json:"clientId,omitempty"`
	Side           string  `json:"side,omitempty"`
	Price          float64 `json:"price,omitempty"`
	Amount         float64 `json:"amount,omitempty"`
	Underlying     string  `json:"underlying,omitempty"`
	ExpirationDate string  `json:"expiryDate,omitempty"`
	StrikePrice    float64 `json:"strikePrice,omitempty"`
	Type           string  `json:"type,omitempty"`
	Contracts      string  `json:"contracts,omitempty"`
}

func newApplication() *Application {

	mongoDb, _ := _mongo.InitConnection(os.Getenv("MONGO_URL"))
	orderRepo := repositories.NewOrderRepository(mongoDb)
	tradeRepo := repositories.NewTradeRepository(mongoDb)
	userRepo := repositories.NewUserRepository(mongoDb)

	app := &Application{
		MessageRouter:   quickfix.NewMessageRouter(),
		UserRepository:  userRepo,
		OrderRepository: orderRepo,
		TradeRepository: tradeRepo,
	}
	app.AddRoute(newordersingle.Route(app.onNewOrderSingle))
	app.AddRoute(ordercancelrequest.Route(app.onOrderCancelRequest))
	app.AddRoute(marketdatarequest.Route(app.onMarketDataRequest))
	app.AddRoute(ordercancelreplacerequest.Route(app.onOrderUpdateRequest))
	app.AddRoute(securitylistrequest.Route(app.onSecurityListRequest))
	return app
}

// OnCreate implemented as part of Application interface
func (a Application) OnCreate(sessionID quickfix.SessionID) {}

// OnLogon implemented as part of Application interface
func (a Application) OnLogon(sessionID quickfix.SessionID) {}

// OnLogout implemented as part of Application interface
func (a Application) OnLogout(sessionID quickfix.SessionID) {}

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
			return err
		}

		if err := msg.Body.Get(&uname); err != nil {
			return err
		}

		// user, err := a.UserRepository.FindByAPIKeyAndSecret(context.TODO(), uname.String(), pwd.String())
		// if err != nil {
		// 	return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
		// }

		if userSession == nil {
			userSession = make(map[string]*quickfix.SessionID)
		}
		userSession["645db1b2533b4f1cd204998c"] = &sessionID
	}
	return nil
}

// FromApp implemented as part of Application interface, uses Router on incoming application messages
func (a *Application) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	return a.Route(msg, sessionID)
}

func (a *Application) onNewOrderSingle(msg newordersingle.NewOrderSingle, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Println("incoming new order")
	if userSession == nil {
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
		return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
	}

	clOrId, err := msg.GetClOrdID()
	if err != nil {
		return err
	}

	symbol, err := msg.GetSymbol()
	if err != nil {
		return err
	}

	side, err := msg.GetSide()
	if err != nil {
		return err
	}

	ordType, err := msg.GetOrdType()
	if err != nil {
		return err
	}

	price, err := msg.GetPrice()
	if err != nil {
		return err
	}

	orderQty, err := msg.GetOrderQty()
	if err != nil {
		return err
	}

	details := strings.Split(symbol, "-")
	underlying := details[0]
	expiryDate := details[1]
	strikePrice := details[2]
	options := details[3]

	var partyId quickfix.FIXString
	err = msg.GetField(tag.PartyID, &partyId)
	if err != nil {
		return err
	}

	strType := "LIMIT"
	if ordType == enum.OrdType_MARKET {
		strType = "MARKET"
	}

	putOrCall := "CALL"
	if options == string(enum.PutOrCall_PUT) {
		putOrCall = "PUT"
	}

	sideStr := "BUY"
	if side == enum.Side_SELL {
		sideStr = "SELL"
	}
	strikePriceFloat, _ := strconv.ParseFloat(strikePrice, 64)
	priceFloat, _ := strconv.ParseFloat(price.String(), 64)
	amountFloat, _ := strconv.ParseFloat(orderQty.String(), 64)
	data := KafkaOrder{
		ClOrdID:        clOrId,
		ClientID:       partyId.String(),
		UserID:         user.ID.Hex(),
		Underlying:     underlying,
		ExpirationDate: expiryDate,
		StrikePrice:    strikePriceFloat,
		Type:           strType,
		Side:           sideStr,
		Price:          priceFloat,
		Amount:         amountFloat,
		Contracts:      string(putOrCall),
	}

	_data, _ := json.Marshal(data)
	_producer.KafkaProducer(string(_data), "NEW_ORDER")

	return nil
}

func (a Application) broadcastInstrumentList(currency string) {
	fmt.Println("broadcastInstrumentList", currency)
	for _, subs := range xMessagesSubs {
		a.SecurityListResponse(currency, subs.secReq, subs.sessiondID)
	}
}

func (a *Application) onOrderUpdateRequest(msg ordercancelreplacerequest.OrderCancelReplaceRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Println("onOrderUpdateRequest")
	userId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			userId = i
		}
	}

	user, e := a.UserRepository.FindById(context.TODO(), userId)
	if e != nil {
		return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
	}

	price, err := msg.GetPrice()
	if err != nil {
		fmt.Println("Error getting price")
		return err
	}

	ordType, err := msg.GetOrdType()
	if err != nil {
		return err
	}

	amount, err := msg.GetOrderQty()
	if err != nil {
		fmt.Println("Error getting amount")
		return err
	}
	orderId, err := msg.GetOrderID()
	if err != nil {
		fmt.Println("Error getting orderid")
		return err
	}

	strType := "LIMIT"
	if ordType == enum.OrdType_MARKET {
		strType = "MARKET"
	}

	symbol, err := msg.GetSymbol()
	if err != nil {
		fmt.Println("Error getting symbol")
		return err
	}

	details := strings.Split(symbol, "-")
	underlying := details[0]
	expiryDate := details[1]
	strikePrice := details[2]
	options := details[3]

	putOrCall := "CALL"
	if options == string(enum.PutOrCall_PUT) {
		putOrCall = "PUT"
	}
	amountFloat, _ := amount.Float64()
	priceFloat, _ := price.Float64()
	strikePriceFloat, _ := strconv.ParseFloat(strikePrice, 64)

	var partyId quickfix.FIXString
	msg.GetField(tag.PartyID, &partyId)

	data := KafkaOrder{
		ID:             orderId,
		ClientID:       partyId.String(),
		UserID:         user.ID.Hex(),
		Amount:         amountFloat,
		Price:          priceFloat,
		Side:           "EDIT",
		Underlying:     underlying,
		ExpirationDate: expiryDate,
		StrikePrice:    strikePriceFloat,
		Type:           string(strType),
		Contracts:      string(putOrCall),
	}

	_data, _ := json.Marshal(data)
	fmt.Println(_data)
	_producer.KafkaProducer(string(_data), "NEW_ORDER")

	return nil
}

func (a *Application) onOrderCancelRequest(msg ordercancelrequest.OrderCancelRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	userId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			userId = i
		}
	}

	user, e := a.UserRepository.FindById(context.TODO(), userId)
	if e != nil {
		return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
	}

	orderId, err := msg.GetOrderID()
	if err != nil {
		return err
	}

	clOrdID, err := msg.GetClOrdID()
	if err != nil {
		return err
	}

	var partyId quickfix.FIXString
	msg.GetField(tag.PartyID, &partyId)

	data := KafkaOrder{
		ID:       orderId,
		ClOrdID:  clOrdID,
		ClientID: partyId.String(),
		UserID:   user.ID.Hex(),
		Side:     "CANCEL",
	}
	if err := quickfix.SendToTarget(msg, sessionID); err != nil {
		return quickfix.NewMessageRejectError("Failed to send cancel request", 1, nil)
	}
	_data, _ := json.Marshal(data)
	fmt.Println(string(_data))
	_producer.KafkaProducer(string(_data), "NEW_ORDER")

	return nil
}

func (a *Application) onMarketDataRequest(msg marketdatarequest.MarketDataRequest, sessionID quickfix.SessionID) (err quickfix.MessageRejectError) {
	fmt.Println("onMarketDataRequest")
	subs, _ := msg.GetSubscriptionRequestType()
	if subs == enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES { // subscribe
		vMessageSubs = append(vMessageSubs, VMessageSubscriber{
			sessiondID: sessionID,
		})
	} else if subs == enum.SubscriptionRequestType_DISABLE_PREVIOUS_SNAPSHOT_PLUS_UPDATE_REQUEST { // unsubscribe
		for _, subs := range vMessageSubs {
			if subs.sessiondID.String() == sessionID.String() {
				vMessageSubs = removeVMessageSubscriber(vMessageSubs, subs)
			}
		}
	}

	mdEntryTypes := marketdatarequest.NewNoMDEntryTypesRepeatingGroup()
	err = msg.GetGroup(mdEntryTypes)
	if err != nil {
		fmt.Println("Error getting mdEntryTypes", err)
	}

	noRelatedsym, _ := msg.GetNoRelatedSym()

	// loop based on symbol requested
	for i := 0; i < noRelatedsym.Len(); i++ {
		response := []MarketDataResponse{}
		sym, _ := noRelatedsym.Get(i).GetSymbol()
		entries := make([]string, mdEntryTypes.Len())
		for j := 0; j < mdEntryTypes.Len(); j++ {
			entry, _ := mdEntryTypes.Get(j).GetMDEntryType()
			entries = append(entries, string(entry))
		}

		if utils.ArrContains(entries, "0") {
			asks := a.OrderRepository.GetMarketData(sym, "BUY")
			for _, ask := range asks {
				if ask.Status == "FILLED" {
					continue
				}
				response = append(response, MarketDataResponse{
					Price:  ask.Price,
					Amount: ask.Amount - ask.FilledAmount,
					Side:   ask.Side,
					InstrumentName: ask.Underlying + "-" + ask.ExpirationDate + "-" + strconv.FormatFloat(ask.StrikePrice, 'f', 0, 64) +
						"-" + ask.Contracts[0:1],
					Date: ask.CreatedAt.String(),
					Type: "ASK",
				})
			}
		}

		if utils.ArrContains(entries, "1") {
			bids := a.OrderRepository.GetMarketData(sym, "SELL")
			for _, bid := range bids {
				if bid.Status == "FILLED" {
					continue
				}
				response = append(response, MarketDataResponse{
					Price:  bid.Price,
					Amount: bid.Amount - bid.FilledAmount,
					Side:   bid.Side,
					InstrumentName: bid.Underlying + "-" + bid.ExpirationDate + "-" + strconv.FormatFloat(bid.StrikePrice, 'f', 0, 64) +
						"-" + bid.Contracts[0:1],
					Date: bid.CreatedAt.String(),
					Type: "BID",
				})
			}

		}

		if utils.ArrContains(entries, "2") {
			splits := strings.Split(sym, "-")
			price, _ := strconv.ParseFloat(splits[2], 64)
			trades, _ := a.TradeRepository.Find(bson.M{
				"underlying":  splits[0],
				"expiryDate":  splits[1],
				"strikePrice": price,
			}, nil, 0, -1)
			for _, trade := range trades {
				response = append(response, MarketDataResponse{
					Price:  trade.Price,
					Amount: trade.Amount,
					Side:   string(trade.Side),
					InstrumentName: trade.Underlying + "-" + trade.ExpiryDate + "-" + strconv.FormatFloat(trade.StrikePrice, 'f', 0, 64) +
						"-" + string(trade.Contracts)[0:1],
					Date:    trade.CreatedAt.String(),
					Type:    "TRADE",
					MakerID: trade.MakerOrderID.Hex(),
					TakerID: trade.TakerOrderID.Hex(),
					Status:  string(trade.Status),
				})
			}
		}

		if len(response) == 0 {
			continue
		}
		snap := marketdatasnapshotfullrefresh.New()
		snap.SetSymbol(response[0].InstrumentName)
		reqId, _ := msg.GetMDReqID()
		snap.SetMDReqID(reqId)
		grp := marketdatasnapshotfullrefresh.NewNoMDEntriesRepeatingGroup()
		response = mapMarketDataResponse(response)
		fmt.Println("response", response)
		for _, res := range response {
			row := grp.Add()
			row.SetMDEntryType(enum.MDEntryType(res.Side))
			row.SetMDEntrySize(decimal.NewFromFloat(res.Amount), 2)
			row.SetMDEntryPx(decimal.NewFromFloat(res.Price), 2)
			row.SetMDEntryDate(res.Date)

			//trade
			if res.Type == "TRADE" {
				side := field.NewSide(enum.Side(res.Side))
				amount := field.NewQuantity(decimal.NewFromFloat(res.Amount), 10)
				status := field.NewStatusText(res.Status)
				orderId := field.NewOrderID(res.MakerID)
				secondaryOrderId := field.NewOrderID(res.TakerID)

				row.Set(side)
				row.Set(amount)
				row.Set(status)
				row.Set(orderId)
				row.Set(secondaryOrderId)
			}

		}
		snap.SetNoMDEntries(grp)
		error := quickfix.SendToTarget(snap, sessionID)
		fmt.Println("replying to market data request")
		if error != nil {
			fmt.Println(error.Error())
		}
	}

	return
}

func mapMarketDataResponse(res []MarketDataResponse) []MarketDataResponse {
	result := []MarketDataResponse{}

	fmt.Println("total data", len(res))
	for i, r := range res {
		if len(result) == 0 {
			result = append(result, r)
			continue
		}
		result, exist := isInstrumentExists(result, r)
		fmt.Println("exist", exist, result, i)
		if !exist {
			fmt.Println("adding ", r.Price, r.Side)
			result = append(result, r)
			fmt.Println("result inside", result)
			if i == len(res)-1 {
				return result
			}
		}
		fmt.Println("result inside 2", result)
	}
	fmt.Println("result outside", result)
	return result
}

func isInstrumentExists(data []MarketDataResponse, marketData MarketDataResponse) ([]MarketDataResponse, bool) {
	fmt.Println("checkingg...", marketData.Side, marketData.Price)
	for i, d := range data {
		if d.InstrumentName == marketData.InstrumentName && d.Price == marketData.Price && d.Side == marketData.Side {
			data[i].Amount = data[i].Amount + marketData.Amount
			return data, true
		}
	}
	return data, false
}

func OnMatchingOrder(data types.EngineResponse) {
	fmt.Println("OnMatchingOrder")
	if data.Matches == nil {
		return
	}

	if data.Matches.Trades == nil {
		return
	}

	for _, trd := range data.Matches.MakerOrders {
		if userSession == nil {
			return
		}
		if userSession[trd.Order.UserID] == nil {
			return
		}

		sessionID := userSession[data.Matches.Trades[0].MakerID]
		if sessionID == nil {
			return
		}
		order := data.Matches.TakerOrder
		msg := executionreport.New(
			field.NewOrderID(trd.ID.String()),
			field.NewExecID(order.ClOrdID),
			field.NewExecType(enum.ExecType(order.Status)),
			field.NewOrdStatus(enum.OrdStatus(order.Status)),
			field.NewSide(enum.Side(trd.Side)),
			field.NewLeavesQty(decimal.NewFromFloat(trd.Amount), 2),
			field.NewCumQty(decimal.NewFromFloat(order.FilledAmount), 2),
			field.NewAvgPx(decimal.NewFromFloat(trd.Price), 2),
		)
		if trd.Amount == 0 {
			msg.SetOrdStatus(enum.OrdStatus_FILLED)
		} else {
			msg.SetOrdStatus(enum.OrdStatus_PARTIALLY_FILLED)
		}
		msg.SetClOrdID(trd.ClOrdID)
		msg.SetLastPx(decimal.NewFromFloat(trd.Price), 2)
		msg.SetLastQty(decimal.NewFromFloat(trd.Amount), 2)
		fmt.Println("Sending execution report for matching order")
		err := quickfix.SendToTarget(msg, *sessionID)
		if err != nil {
			fmt.Println("Error sending execution report")
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
		quickfix.SendToTarget(msg, sess.sessiondID)
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
		fmt.Println(sendErr)
	}

}

func OrderConfirmation(userId string, order Order, symbol string) {
	if userSession == nil {
		if userSession[userId] == nil {
			return
		}
		return
	}
	sessionId := userSession[userId]
	exec := 0
	switch order.Status {
	case "FILLED":
		exec = 2
		break
	case "PARTIALLY FILLED":
		exec = 1
		break
	}

	msg := executionreport.New(
		field.NewOrderID(order.ClientOrderId),
		field.NewExecID(strconv.Itoa(exec)),
		field.NewExecType(enum.ExecType(order.Status)),
		field.NewOrdStatus(enum.OrdStatus(order.Status)),
		field.NewSide(enum.Side(order.Side)),
		field.NewLeavesQty(order.Amount.Sub(order.FilledAmount), 2),
		field.NewCumQty(order.FilledAmount, 2),
		field.NewAvgPx(order.Price, 2),
	)
	msg.SetOrdStatus(enum.OrdStatus_NEW)
	msg.SetString(tag.OrderID, order.ID)
	msg.SetString(tag.ClOrdID, order.ClientOrderId)

	if sessionId == nil {
		return
	}
	err := quickfix.SendToTarget(msg, *sessionId)
	if err != nil {
		fmt.Print(err.Error())
	}
	fmt.Println("new order, send instruments")
	newApplication().broadcastInstrumentList(order.Underlying)
}

func (a Application) onSecurityListRequest(msg securitylistrequest.SecurityListRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Println("receiving security list request")
	secReq, err := msg.GetSecurityReqID()
	if err != nil {
		return err
	}

	currency, err := msg.GetCurrency()
	if err != nil {
		return err
	}

	subs, err := msg.GetSubscriptionRequestType()
	if err != nil {
		return err
	}

	if subs == enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES {
		xMessagesSubs = append(xMessagesSubs, XMessageSubscriber{
			sessiondID: sessionID,
			secReq:     secReq,
		})
	} else if subs == enum.SubscriptionRequestType_DISABLE_PREVIOUS_SNAPSHOT_PLUS_UPDATE_REQUEST {
		for _, x := range xMessagesSubs {
			if x.sessiondID == sessionID {
				xMessagesSubs = removeXMessageSubscriber(xMessagesSubs, x)
			}
		}
	}

	fmt.Println("requesting", subs)
	fmt.Println("requesting", xMessagesSubs)
	fmt.Println("currency", currency)
	err = a.SecurityListResponse(currency, secReq, sessionID)
	if err != nil {
		return err
	}
	return nil
}

func removeXMessageSubscriber(array []XMessageSubscriber, element XMessageSubscriber) []XMessageSubscriber {
	for i, v := range array {
		if v == element {
			return append(array[:i], array[i+1:]...)
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

func (a Application) SecurityListResponse(currency string, secReq string, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Println("sending security list response")
	secRes := time.Now().UnixMicro()
	res := securitylist.New(
		field.NewSecurityReqID(secReq),
		field.NewSecurityResponseID(strconv.Itoa(int(secRes))),
		field.NewSecurityRequestResult(enum.SecurityRequestResult_VALID_REQUEST),
	)

	// get isntrument from mongo
	instruments, e := a.OrderRepository.GetAvailableInstruments(currency)
	if e != nil {
		return quickfix.NewMessageRejectError(e.Error(), 0, nil)
	}

	fmt.Println("instrumentsz", instruments)
	secListGroup := securitylist.NewNoRelatedSymRepeatingGroup()
	for _, instrument := range instruments {
		row := secListGroup.Add()
		strikePrice := strconv.FormatFloat(instrument.StrikePrice, 'f', 0, 64)
		instrumentName := fmt.Sprintf("%s-%s-%s-%s", instrument.Underlying, instrument.ExpirationDate, strikePrice, instrument.Contracts)
		row.SetSymbol(instrumentName)

		row.SetSecurityDesc("OPTIONS")
		row.SetSecurityType("OPT")
		row.SetStrikePrice(decimal.NewFromFloat(instrument.StrikePrice), 0)
		row.SetStrikeCurrency("USD")

	}

	res.SetNoRelatedSym(secListGroup)
	fmt.Println(res.ToMessage().String())
	fmt.Println("giving back security list msg")
	quickfix.SendToTarget(res, sessionID)
	return nil
}

const (
	usage = "ordermatch"
	short = "Start an order matching (FIX acceptor) service"
	long  = "Start an order matching (FIX acceptor) service."
)

var (
	// Cmd is the quote command.
	Cmd = &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    long,
		Aliases: []string{"oms"},
		Example: "qf ordermatch [YOUR_FIX_CONFIG_FILE_HERE.cfg] (default is ./config/cfg)",
		RunE:    execute,
	}
)

func execute(cmd *cobra.Command, args []string) error {
	cfgFileName := "ordermatch.cfg"
	templateCfg := "ordermatch_template.cfg"
	_, b, _, _ := runtime.Caller(0)

	input, _ := ioutil.ReadFile(path.Join(b, "../", "config", templateCfg))

	config := strings.Replace(string(input), "$DATA_DICTIONARY_PATH", os.Getenv("DATA_DICTIONARY_PATH"), 1)

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
	app := newApplication()
	utils.PrintConfig("acceptor", bytes.NewReader(stringData))
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
