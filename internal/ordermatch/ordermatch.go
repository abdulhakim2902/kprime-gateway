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
	"syscall"

	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix42/executionreport"
	"github.com/quickfixgo/fix42/marketdatarequest"
	"github.com/quickfixgo/fix42/newordersingle"
	"github.com/quickfixgo/fix42/ordercancelrequest"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	_producer "gateway/pkg/kafka/producer"

	"github.com/quickfixgo/quickfix"
)

// Application implements the quickfix.Application interface
type Application struct {
	*quickfix.MessageRouter
	*OrderMatcher
	execID int
	*gorm.DB
}

type KafkaOrder struct {
	UserID         string    `json:"user_id"`
	ClientID       string    `json:"client_id"`
	Symbol         string    `json:"symbol"`
	Side           enum.Side `json:"side"`
	Price          float64   `json:"price"`
	Amount         float64   `json:"quantity"`
	Underlying     string    `json:"underlying"`
	ExpirationDate string    `json:"expiration_date"`
	StrikePrice    string    `json:"strike_price"`
	Type           string    `json:"type"`
}

func newApplication() *Application {
	dsn := os.Getenv("DB_CONNECTION")
	db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	app := &Application{
		MessageRouter: quickfix.NewMessageRouter(),
		OrderMatcher:  NewOrderMatcher(),
		DB:            db,
	}
	app.AddRoute(newordersingle.Route(app.onNewOrderSingle))
	app.AddRoute(ordercancelrequest.Route(app.onOrderCancelRequest))
	app.AddRoute(marketdatarequest.Route(app.onMarketDataRequest))

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
			Name: uname.String(),
		}).Find(&user)

		if res.Error != nil {
			return quickfix.NewMessageRejectError("Failed getting user", 1, nil)
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.APISecret), []byte(pwd.String())); err != nil {
			// return quickfix.NewMessageRejectError("Wrong API Secret", 1, nil)
		}
	}
	return nil
}

// FromApp implemented as part of Application interface, uses Router on incoming application messages
func (a *Application) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	return a.Route(msg, sessionID)
}

func (a *Application) onNewOrderSingle(msg newordersingle.NewOrderSingle, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	clOrdID, err := msg.GetClOrdID()
	if err != nil {
		return err
	}

	symbol, err := msg.GetSymbol()
	if err != nil {
		return err
	}

	senderCompID, err := msg.Header.GetSenderCompID()
	if err != nil {
		return err
	}

	targetCompID, err := msg.Header.GetTargetCompID()
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

	order := Order{
		ClOrdID:      clOrdID,
		Symbol:       symbol,
		SenderCompID: senderCompID,
		TargetCompID: targetCompID,
		Side:         side,
		OrdType:      ordType,
		Price:        price,
		Quantity:     orderQty,
	}

	data := KafkaOrder{
		UserID:   order.SenderCompID,
		ClientID: order.TargetCompID,
		Symbol:   order.Symbol,
		Side:     order.Side,
		Price:    order.Price.InexactFloat64(),
		Amount:   order.Quantity.Tan().Copy().InexactFloat64(),
		Type:     string(ordType),
	}

	_data, _ := json.Marshal(data)
	fmt.Println(data)
	_producer.KafkaProducer(string(_data), "ORDER")
	return nil
}

func (a *Application) onOrderCancelRequest(msg ordercancelrequest.OrderCancelRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	origClOrdID, err := msg.GetOrigClOrdID()
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

	order := a.Cancel(origClOrdID, symbol, side)
	if order != nil {
		a.cancelOrder(*order)
	}

	return nil
}

func (a *Application) onMarketDataRequest(msg marketdatarequest.MarketDataRequest, sessionID quickfix.SessionID) (err quickfix.MessageRejectError) {
	fmt.Printf("%+v\n", msg)
	return
}

func (a *Application) acceptOrder(order Order) {
	a.updateOrder(order, enum.OrdStatus_NEW)
}

func (a *Application) fillOrder(order Order) {
	status := enum.OrdStatus_FILLED
	if !order.IsClosed() {
		status = enum.OrdStatus_PARTIALLY_FILLED
	}
	a.updateOrder(order, status)
}

func (a *Application) cancelOrder(order Order) {
	a.updateOrder(order, enum.OrdStatus_CANCELED)
}

func (a *Application) genExecID() string {
	a.execID++
	return strconv.Itoa(a.execID)
}

func (a *Application) updateOrder(order Order, status enum.OrdStatus) {
	execReport := executionreport.New(
		field.NewOrderID(order.ClOrdID),
		field.NewExecID(a.genExecID()),
		field.NewExecTransType(enum.ExecTransType_NEW),
		field.NewExecType(enum.ExecType(status)),
		field.NewOrdStatus(status),
		field.NewSymbol(order.Symbol),
		field.NewSide(order.Side),
		field.NewLeavesQty(order.OpenQuantity(), 2),
		field.NewCumQty(order.ExecutedQuantity, 2),
		field.NewAvgPx(order.AvgPx, 2),
	)
	execReport.SetOrderQty(order.Quantity, 2)
	execReport.SetClOrdID(order.ClOrdID)

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
