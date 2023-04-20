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
	"encoding/json"
	"fmt"
	"gateway/internal/user/model"
	"gateway/pkg/utils"
	"io"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix42/executionreport"
	"github.com/quickfixgo/fix42/marketdatarequest"
	"github.com/quickfixgo/fix42/marketdatasnapshotfullrefresh"
	"github.com/quickfixgo/fix42/newordersingle"
	"github.com/quickfixgo/fix42/ordercancelreplacerequest"
	"github.com/quickfixgo/fix42/ordercancelrequest"
	"github.com/quickfixgo/tag"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	_producer "gateway/pkg/kafka/producer"

	"github.com/quickfixgo/quickfix"
)

var userSession map[string]*quickfix.SessionID
var orderSubs map[string][]quickfix.SessionID

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

type Orderbook struct {
	InstrumentName string   `json:"instrumentName" bson:"instrumentName"`
	Bids           []*Order `json:"bids" bson:"bids"`
	Asks           []*Order `json:"asks" bson:"asks"`
}

// Application implements the quickfix.Application interface
type Application struct {
	*quickfix.MessageRouter
	execID int
	*gorm.DB
}

type KafkaOrder struct {
	ID             string    `json:"id"`
	ClOrdID        string    `json:"clOrdID,omitempty"`
	UserID         string    `json:"userId,omitempty"`
	ClientID       string    `json:"clientId,omitempty"`
	Side           enum.Side `json:"side,omitempty"`
	Price          float64   `json:"price,omitempty"`
	Amount         float64   `json:"amount,omitempty"`
	Underlying     string    `json:"underlying,omitempty"`
	ExpirationDate string    `json:"expiryDate,omitempty"`
	StrikePrice    float64   `json:"strikePrice,omitempty"`
	Type           string    `json:"type,omitempty"`
	Contracts      string    `json:"contracts,omitempty"`
}

func newApplication() *Application {
	dsn := os.Getenv("DB_CONNECTION")
	db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	app := &Application{
		MessageRouter: quickfix.NewMessageRouter(),
		DB:            db,
	}
	app.AddRoute(newordersingle.Route(app.onNewOrderSingle))
	app.AddRoute(ordercancelrequest.Route(app.onOrderCancelRequest))
	app.AddRoute(marketdatarequest.Route(app.onMarketDataRequest))
	app.AddRoute(ordercancelreplacerequest.Route(app.onOrderUpdateRequest))

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
		var user model.Client
		res := a.DB.Where(model.Client{
			APIKey: uname.String(),
		}).Find(&user)

		if res.Error != nil {
			return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.APISecret), []byte(pwd.String())); err != nil {
			return quickfix.NewMessageRejectError("Wrong API Secret", 1, nil)
		}

		if userSession == nil {
			userSession = make(map[string]*quickfix.SessionID)
		}
		userSession[strconv.Itoa(int(user.ID))] = &sessionID

	}
	return nil
}

// FromApp implemented as part of Application interface, uses Router on incoming application messages
func (a *Application) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	return a.Route(msg, sessionID)
}

func (a *Application) onNewOrderSingle(msg newordersingle.NewOrderSingle, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	if userSession == nil {
		return quickfix.NewMessageRejectError("User not logged in", 1, nil)
	}
	clientId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			clientId = i
		}
	}

	strClientId, _ := strconv.Atoi(clientId)
	var client model.Client
	res := a.DB.Where(model.Client{ID: uint(strClientId)}).Find(&client)
	if res.Error != nil {
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

	side = "BUY"
	if side == enum.Side_SELL {
		side = "SELL"
	}
	strikePriceFloat, _ := strconv.ParseFloat(strikePrice, 64)
	priceFloat, _ := strconv.ParseFloat(price.String(), 64)
	amountFloat, _ := strconv.ParseFloat(orderQty.String(), 64)
	data := KafkaOrder{
		ClOrdID:        clOrId,
		ClientID:       partyId.String(),
		UserID:         strconv.Itoa(int(client.ID)),
		Underlying:     underlying,
		ExpirationDate: expiryDate,
		StrikePrice:    strikePriceFloat,
		Type:           string(strType),
		Side:           side,
		Price:          priceFloat,
		Amount:         amountFloat,
		Contracts:      string(putOrCall),
	}

	_data, _ := json.Marshal(data)
	_producer.KafkaProducer(string(_data), "NEW_ORDER")
	return nil
}

func (a *Application) onOrderUpdateRequest(msg ordercancelreplacerequest.OrderCancelReplaceRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Println("onOrderUpdateRequest")
	clientId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			clientId = i
		}
	}

	strClientId, _ := strconv.Atoi(clientId)
	var client model.Client
	res := a.DB.Where(model.Client{ID: uint(strClientId)}).Find(&client)
	if res.Error != nil {
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
		UserID:         strconv.Itoa(int(client.ID)),
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
	clientId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			clientId = i
		}
	}

	strClientId, _ := strconv.Atoi(clientId)
	var client model.Client
	res := a.DB.Where(model.Client{ID: uint(strClientId)}).Find(&client)
	if res.Error != nil {
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
		UserID:   strconv.Itoa(int(client.ID)),
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
	fmt.Printf("%+v\n", msg)
	var symbol field.SymbolField
	msg.Body.GetField(quickfix.Tag(55), &symbol)
	subs, _ := msg.Body.GetInt(quickfix.Tag(263))
	if subs == 1 { // subscribe
		orderSubs[symbol.String()] = append(orderSubs[symbol.String()], sessionID)
	} else if subs == 2 { // unsubscribe
		for i, sess := range orderSubs[symbol.String()] {
			if sess == sessionID {
				orderSubs[symbol.String()] = append(orderSubs[symbol.String()][:i], orderSubs[symbol.String()][i+1:]...)
			}
		}
	}
	return
}

func OnOrderboookUpdate(symbol string, data map[string]interface{}) {

	bids := data["bids"].([]Order)
	asks := data["asks"].([]Order)

	msg := marketdatasnapshotfullrefresh.New(field.SymbolField{quickfix.FIXString(symbol)})

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

	for _, sess := range orderSubs[symbol] {
		quickfix.SendToTarget(msg, sess)
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
		field.NewExecTransType(enum.ExecTransType_NEW),
		field.NewExecType(enum.ExecType(status)),
		field.NewOrdStatus(status),
		field.NewSymbol(order.Symbol),
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
		execReport.SetLastShares(order.LastExecutedQuantity, 2)
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
		field.NewExecTransType(enum.ExecTransType_NEW),
		field.NewExecType(enum.ExecType(order.Status)),
		field.NewOrdStatus(enum.OrdStatus(order.Status)),
		field.NewSymbol(symbol),
		field.NewSide(enum.Side(order.Side)),
		field.NewLeavesQty(order.Amount.Sub(order.FilledAmount), 2),
		field.NewCumQty(order.FilledAmount, 2),
		field.NewAvgPx(order.Price, 2),
	)
	msg.SetString(tag.OrderID, order.ID)
	msg.SetString(tag.ClOrdID, order.ClientOrderId)
	err := quickfix.SendToTarget(msg, *sessionId)
	if err != nil {
		fmt.Print(err.Error())
	}
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
	var cfgFileName string

	_, b, _, _ := runtime.Caller(0)
	cfg, err := os.Open(path.Join(b, "../", "config", "ordermatch.cfg"))
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

	logger := utils.NewFancyLog()
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
