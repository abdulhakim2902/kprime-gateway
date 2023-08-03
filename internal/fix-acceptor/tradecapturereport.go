package ordermatch

import (
	"context"
	"fmt"
	"gateway/internal/engine/types"
	_orderbookType "gateway/internal/orderbook/types"
	_userType "gateway/internal/user/types"
	"gateway/pkg/constant"
	"gateway/pkg/utils"
	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/tradecapturereport"
	"github.com/quickfixgo/fix44/tradecapturereportrequest"
	"github.com/quickfixgo/quickfix"
	"github.com/shopspring/decimal"
)

func ADSubscribe(symbol string, sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	if subsManager.ADSubscriptions[symbol] == nil {
		subsManager.ADSubscriptions[symbol] = make(map[quickfix.SessionID]bool)
	}

	subsManager.ADSubscriptions[symbol][sessionID] = true

	if subsManager.ADSubscriptionsList[sessionID] == nil {
		subsManager.ADSubscriptionsList[sessionID] = []string{}
	}

	subsManager.ADSubscriptionsList[sessionID] = append(subsManager.ADSubscriptionsList[sessionID], symbol)
}

func ADUnsubscribeAll(sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	symbols := subsManager.ADSubscriptionsList[sessionID]
	if symbols == nil {
		return
	}

	for _, id := range subsManager.ADSubscriptionsList[sessionID] {
		if subsManager.ADSubscriptions[id][sessionID] {
			subsManager.ADSubscriptions[id][sessionID] = false
			delete(subsManager.ADSubscriptions[id], sessionID)
		}
	}
}

func ADUnsubscribe(symbol string, sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	if subsManager.ADSubscriptions[symbol][sessionID] {
		subsManager.ADSubscriptions[symbol][sessionID] = false
		delete(subsManager.ADSubscriptions[symbol], sessionID)
	}
}

func (a *Application) OnTradeCaptureReportRequestSubscribe(symbol string, sessionID quickfix.SessionID) string {
	ADSubscribe(symbol, sessionID)
	return ""
}

func (a *Application) OnTradeCaptureReportRequestUnsubscribe(symbol string, sessionID quickfix.SessionID) string {
	ADUnsubscribe(symbol, sessionID)
	return ""
}

func (a *Application) BroadcastTradeCaptureReport(trades []*types.Trade) {
	// check whether trades is not empty array
	if len(trades) == 0 {
		return
	}

	for _, trade := range trades {
		symbol := trade.Underlying + "-" + trade.ExpiryDate + "-" + fmt.Sprintf("%.0f", trade.StrikePrice) + "-" + string(trade.Contracts[0])
		conversion, _ := utils.ConvertToFloat(trade.Amount)
		msg := tradecapturereport.New(
			field.NewTradeReportID("notification"), // Need Req ID
			field.NewPreviouslyReported(false),
			field.NewLastQty(decimal.NewFromFloat(conversion), 2),    // decimal
			field.NewLastPx(decimal.NewFromFloat(trade.Price), 2),    // decimal
			field.NewTradeDate(trade.CreatedAt.Format("2006-01-02")), // string YYYYMMDD
			field.NewTransactTime(trade.CreatedAt),                   // time
		)
		msg.SetSymbol(symbol)

		// We did not implement sides (side and order ID)
		grp := tradecapturereport.NewNoSidesRepeatingGroup()
		row := grp.Add()
		row.SetSide(enum.Side_BUY)
		row.SetOrderID("")
		msg.SetNoSides(grp)

		for sessionID, status := range subsManager.ADSubscriptions[symbol] {
			if status {
				err := quickfix.SendToTarget(msg, sessionID)
				if err != nil {
					logs.Log.Err(err).Msg("Error sending trade capture request update")
				}
			}
		}
	}
}

// MsgType AD
// https://www.onixs.biz/fix-dictionary/4.4/msgtype_ad_6568.html#:~:text=Description,the%20trade%20capture%20report%20request.
// Required tags:
// 568 TradeRequestID
// 569 TradeRequestType = always 0
// 263 SubscriptionRequestType = can be 0 snapshot, 1 subscribe, 2 unsubscribe
// 55 Symbol
// Response
// Required tags:
// 571 TradeReportID
// 32 LastQty
// 31 LastPx
// 55 Symbol
func (a *Application) OnTradeCaptureReportRequest(msg tradecapturereportrequest.TradeCaptureReportRequest, sessionID quickfix.SessionID) (err quickfix.MessageRejectError) {

	// Check user session
	if userSession == nil {
		logs.Log.Err(fmt.Errorf("no session found")).Msg("no session found")
		return quickfix.NewMessageRejectError(constant.NO_SESSION_FOUND, 1, nil)
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
		return quickfix.NewMessageRejectError(constant.NO_USER_FOUND, 1, nil)
	}

	// 568
	tradeRequestID, err := msg.GetTradeRequestID()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting tradeRequestID")
		return err
	}

	// 569
	tradeRequestType, err := msg.GetTradeRequestType()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting tradeRequestType")
		return err
	}

	// 263
	subscriptionRequestType, err := msg.GetSubscriptionRequestType()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting subscriptionRequestType")
		return err
	}

	// 55
	symbol, err := msg.GetSymbol()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting symbol")
		return err
	}

	// Validating requests
	if tradeRequestType != enum.TradeRequestType_ALL_TRADES {
		logs.Log.Err(err).Msg("Invalid tradeRequestType")
		return quickfix.NewMessageRejectError("Invalid tradeRequestType", 1, nil)
	}

	// unsubscribe
	if subscriptionRequestType == enum.SubscriptionRequestType_DISABLE_PREVIOUS_SNAPSHOT_PLUS_UPDATE_REQUEST {
		_ = a.OnTradeCaptureReportRequestUnsubscribe(symbol, sessionID)
		return
	}

	// Snapshot + updates
	if subscriptionRequestType == enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES || subscriptionRequestType == enum.SubscriptionRequestType_SNAPSHOT {
		msg, errStr := a.OnTradeCaptureReportRequestSnapshot(tradeRequestID, user, symbol)
		if errStr != "" {
			return quickfix.NewMessageRejectError(errStr, 1, nil)
		}
		if msg == nil {
			errSnt := quickfix.SendToTarget(msg, sessionID)
			if errSnt != nil {
				logs.Log.Err(errSnt).Msg("Error sending message")
				return quickfix.NewMessageRejectError(constant.ERROR_SENDING_MESSAGE, 1, nil)
			}
		}
	}

	// subscribe
	if subscriptionRequestType == enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES {
		errStr := a.OnTradeCaptureReportRequestSubscribe(symbol, sessionID)
		if errStr != "" {
			return quickfix.NewMessageRejectError(errStr, 1, nil)
		}
	}

	return
}

// Required tags:
// 571 TradeReportID
// 32 LastQty
// 31 LastPx
// 55 Symbol
func (a *Application) OnTradeCaptureReportRequestSnapshot(tradeRequestID string, user *_userType.User, symbol string) (*tradecapturereport.TradeCaptureReport, string) {
	// Create an empty model from _orderbookType.Orderbook
	orderbook := _orderbookType.GetOrderBook{}

	// Split symbol
	instruments, err := utils.ParseInstruments(symbol, false)
	if err != nil {
		logs.Log.Err(err).Msg("Error parsing instruments")
		return nil, constant.INVALID_INSTRUMENT
	}

	orderbook.Underlying = instruments.Underlying
	orderbook.StrikePrice = instruments.Strike
	orderbook.ExpiryDate = instruments.ExpDate

	orderbook.UserRole = user.Role.Name
	ordExclusions := []string{}
	for _, userCast := range user.OrderExclusions {
		ordExclusions = append(ordExclusions, userCast.UserID)
	}
	orderbook.UserOrderExclusions = ordExclusions

	// get last trade from repo
	trades := a.TradeRepository.GetLastTrades(orderbook)
	if len(trades) == 0 {
		return nil, ""
	}

	lastTrade := trades[len(trades)-1]

	conversion, _ := utils.ConvertToFloat(lastTrade.Amount)

	msg := tradecapturereport.New(
		field.NewTradeReportID(tradeRequestID), // Need Req ID
		field.NewPreviouslyReported(false),
		field.NewLastQty(decimal.NewFromFloat(conversion), 2),        // decimal
		field.NewLastPx(decimal.NewFromFloat(lastTrade.Price), 2),    // decimal
		field.NewTradeDate(lastTrade.CreatedAt.Format("2006-01-02")), // string YYYYMMDD
		field.NewTransactTime(lastTrade.CreatedAt),                   // time
	)
	msg.SetSymbol(symbol)

	// We did not implement sides (side and order ID)
	grp := tradecapturereport.NewNoSidesRepeatingGroup()
	row := grp.Add()
	row.SetSide(enum.Side_BUY)
	row.SetOrderID("")
	msg.SetNoSides(grp)

	return &msg, ""
}
