package ordermatch

import (
	"gateway/pkg/utils"
	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/securitylist"
	"github.com/quickfixgo/fix44/securitylistrequest"
	"github.com/quickfixgo/quickfix"
	"github.com/shopspring/decimal"
	"strconv"
	"time"
)

func XSubscribe(sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	if subsManager.XSubscriptions["any"] == nil {
		subsManager.XSubscriptions["any"] = make(map[quickfix.SessionID]bool)
	}

	subsManager.XSubscriptions["any"][sessionID] = true

	if subsManager.XSubscriptionsList[sessionID] == nil {
		subsManager.XSubscriptionsList[sessionID] = []string{}
	}

	subsManager.XSubscriptionsList[sessionID] = append(subsManager.XSubscriptionsList[sessionID], "any")
}

func XUnsubscribeAll(sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	symbols := subsManager.XSubscriptionsList[sessionID]
	if symbols == nil {
		return
	}

	for _, id := range subsManager.XSubscriptionsList[sessionID] {
		if subsManager.XSubscriptions[id][sessionID] {
			subsManager.XSubscriptions[id][sessionID] = false
			delete(subsManager.XSubscriptions[id], sessionID)
		}
	}
}

func XUnsubscribe(sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	if subsManager.XSubscriptions["any"][sessionID] {
		subsManager.XSubscriptions["any"][sessionID] = false
		delete(subsManager.XSubscriptions["any"], sessionID)
	}
}

func (a Application) onSecurityListRequest(msg securitylistrequest.SecurityListRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	// 320
	secReq, err := msg.GetSecurityReqID()
	if err != nil {
		return err
	}

	// 15
	currency, err := msg.GetCurrency()
	if err != nil {
		return err
	}

	// 263
	subs, err := msg.GetSubscriptionRequestType()
	if err != nil {
		return err
	}

	// Unsubscribe
	if subs == enum.SubscriptionRequestType_DISABLE_PREVIOUS_SNAPSHOT_PLUS_UPDATE_REQUEST {
		XUnsubscribe(sessionID)
		return nil
	}

	// Snapshot + Updates
	if subs == enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES || subs == enum.SubscriptionRequestType_SNAPSHOT {
		err = a.SecurityListSnapshot(currency, secReq, sessionID)
		if err != nil {
			logs.Log.Err(err).Msg("Error sending security list response")
			return err
		}
	}

	// Subscribe
	if subs == enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES {
		XSubscribe(sessionID)
	}

	return nil
}

// Response (y) Security List
// 320 SecurityReqID
// 322 SecurityResponseID = {Currency} - {Timestamp}
// 560 SecurityRequestResult = 0 means success
// => 55 Symbol
// => 107 SecurityDesc
// => 167 SecurityType
// => 947 StrikeCurrency (USD)
// => 202 StrikePrice
func (a Application) SecurityListSnapshot(currency string, secReq string, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	secRes := time.Now().UnixMicro()
	res := securitylist.New(
		field.NewSecurityReqID(secReq),                                           // 320
		field.NewSecurityResponseID(strconv.Itoa(int(secRes))),                   // 322
		field.NewSecurityRequestResult(enum.SecurityRequestResult_VALID_REQUEST), // 0
	)

	// Getting User ID
	userId := ""
	for i, v := range userSession {
		if v.String() == sessionID.String() {
			userId = i
		}
	}

	// Get Available Instruments, including expired ones
	instruments, e := a.OrderRepository.GetInstruments(userId, currency, false)
	if e != nil {
		return quickfix.NewMessageRejectError(e.Error(), 0, nil)
	}

	fmt.Println("debug instrument len", len(instruments))
	// Group Responses
	secListGroup := securitylist.NewNoRelatedSymRepeatingGroup()
	i := 1
	for _, instrument := range instruments {
		i++
		if i == 20 {
			break
		}
		row := secListGroup.Add()

		instrumentName := instrument.InstrumentName
		row.SetSymbol(instrumentName)

		row.SetSecurityDesc("OPTIONS")
		row.SetSecurityType("OPT")
		row.SetStrikePrice(decimal.NewFromFloat(instrument.Strike), 0)
		row.SetStrikeCurrency("USD")
	}

	res.SetNoRelatedSym(secListGroup)
	quickfix.SendToTarget(res, sessionID)
	return nil
}

// Handle when ENGINE_SAVED hook being called
func (a Application) BroadcastSecurityList(instrumentName string) {
	// Count the existing security lists
	// If there is no security list, then we will send a new one
	i, err := utils.ParseInstruments(instrumentName, false)
	if err != nil {
		return
	}

	count := a.OrderRepository.GetOrderCountByInstrument(i.Underlying, i.Strike, i.ExpDate, i.Contracts.String())

	// If it is not a new instrument
	if count != 1 {
		return
	}

	secRes := time.Now().UnixMicro()
	msg := securitylist.New(
		field.NewSecurityReqID("notification"),                                   // 320
		field.NewSecurityResponseID(strconv.Itoa(int(secRes))),                   // 322
		field.NewSecurityRequestResult(enum.SecurityRequestResult_VALID_REQUEST), // 0
	)

	// Group Responses
	secListGroup := securitylist.NewNoRelatedSymRepeatingGroup()
	row := secListGroup.Add()
	row.SetSymbol(instrumentName)
	row.SetSecurityDesc("OPTIONS")
	row.SetSecurityType("OPT")
	row.SetStrikePrice(decimal.NewFromFloat(i.Strike), 0)
	row.SetStrikeCurrency("USD")
	msg.SetNoRelatedSym(secListGroup)

	// Broadcast
	for sessionID, status := range subsManager.XSubscriptions["any"] {
		if status {
			err := quickfix.SendToTarget(msg, sessionID)
			if err != nil {
				logs.Log.Err(err).Msg("Error broadcasting security list")
			}
		}
	}
}
