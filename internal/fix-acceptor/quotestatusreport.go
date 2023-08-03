package ordermatch
import (
	"github.com/quickfixgo/quickfix"
	"fmt"
	"gateway/pkg/utils"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/quotestatusrequest"
	"github.com/quickfixgo/fix44/quotestatusreport"
	"github.com/shopspring/decimal"
	"github.com/quickfixgo/enum"
	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	_orderbookType "gateway/internal/orderbook/types"
	"gateway/pkg/constant"

)

func ASubscribe(symbol string, sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	if subsManager.ASubscriptions[symbol] == nil {
		subsManager.ASubscriptions[symbol] = make(map[quickfix.SessionID]bool)
	}

	subsManager.ASubscriptions[symbol][sessionID] = true

	if subsManager.ASubscriptionsList[sessionID] == nil {
		subsManager.ASubscriptionsList[sessionID] = []string{}
	}

	subsManager.ASubscriptionsList[sessionID] = append(subsManager.ASubscriptionsList[sessionID], symbol)
}

func AUnsubscribeAll(sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	symbols := subsManager.ASubscriptionsList[sessionID]
	if symbols == nil {
		return
	}

	for _, id := range subsManager.ASubscriptionsList[sessionID] {
		if subsManager.ASubscriptions[id][sessionID] {
			subsManager.ASubscriptions[id][sessionID] = false
			delete(subsManager.ASubscriptions[id], sessionID)
		}
	}
}

func AUnsubscribe(symbol string, sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	if subsManager.ASubscriptions[symbol][sessionID] {
		subsManager.ASubscriptions[symbol][sessionID] = false
		delete(subsManager.ASubscriptions[symbol], sessionID)
	}
}

// Quote status request (a)
// Required tags:
// 55 symbol
// 263 SubscriptionRequestType
// Response Quote Status Report (AI)
// Required tags:
// MktBidPx 645
// MktOfferPx 646
// QuoteID 117
// Symbol 55
func (a *Application) OnQuoteStatusRequest(msg quotestatusrequest.QuoteStatusRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	symbol, err := msg.GetSymbol()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting symbol")
		return err
	}

	quoteID, err := msg.GetQuoteID()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting Quote ID")
		return err
	}

	// 263
	subscriptionRequestType, err := msg.GetSubscriptionRequestType()
	if err != nil {
		logs.Log.Err(err).Msg("Error getting subscriptionRequestType")
		return err
	}

	// unsubscribe
	if subscriptionRequestType == enum.SubscriptionRequestType_DISABLE_PREVIOUS_SNAPSHOT_PLUS_UPDATE_REQUEST {
		ASubscribe(symbol, sessionID)
		return nil
	}

	// snapshot + updates
	if subscriptionRequestType == enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES  || subscriptionRequestType == enum.SubscriptionRequestType_SNAPSHOT {
		res,errStr := a.OnQuoteStatusRequestSnapshot(symbol, quoteID)
		if errStr != "" {
			return quickfix.NewMessageRejectError(errStr, 1, nil)
		}

		errSnt := quickfix.SendToTarget(res, sessionID)
		if errSnt != nil {
			logs.Log.Err(errSnt).Msg("Error sending message")
			return quickfix.NewMessageRejectError(constant.ERROR_SENDING_MESSAGE, 1, nil)
		}
	}

	// subscribe
	if subscriptionRequestType == enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES {
		ASubscribe(symbol, sessionID)
	}

	return nil
}

func (a *Application) OnQuoteStatusRequestSnapshot(symbol string, responseID string) (*quotestatusreport.QuoteStatusReport, string){

	// Split symbol
	instruments, errGo := utils.ParseInstruments(symbol, false)
	if errGo != nil {
		logs.Log.Err(errGo).Msg("Error parsing instruments")
		return nil, "err"
	}

	orderbook := _orderbookType.GetOrderBook{}
	orderbook.Underlying = instruments.Underlying
	orderbook.StrikePrice = instruments.Strike
	orderbook.ExpiryDate = instruments.ExpDate

	quote, _ := a.WSService.GetDataQuote(orderbook)
	mktAskPrice := quote.BestAskPrice
	mktBidPrice := quote.BestBidPrice
	
	res := quotestatusreport.New(field.NewQuoteID(responseID))
	res.SetMktBidPx(decimal.NewFromFloat(mktBidPrice), 2)
	res.SetMktOfferPx(decimal.NewFromFloat(mktAskPrice), 2)
	res.SetSymbol(symbol)

	return &res, ""
}

func (a *Application) BroadcastQuoteStatusReport(symbol string) {
	msg, err := a.OnQuoteStatusRequestSnapshot(symbol, "notification")

	if err != "" {
		logs.Log.Err(fmt.Errorf("%s", err)).Msg("Error getting quote status report")
		return
	}

	for sessionID, status := range subsManager.ASubscriptions[symbol] {
		if status {
			err := quickfix.SendToTarget(msg, sessionID)
			if err != nil {
				logs.Log.Err(err).Msg("Error sending quote capture request update")
			}
		}
	}
}