package basic

import (
	"fmt"
	"gateway/pkg/utils"
	"log"
	"os"
	"path"
	"runtime"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"github.com/joho/godotenv"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/marketdataincrementalrefresh"
	"github.com/quickfixgo/fix44/marketdatasnapshotfullrefresh"
	"github.com/quickfixgo/fix44/securitylist"
	"github.com/quickfixgo/tag"
	"github.com/quickfixgo/traderui/oms"

	"github.com/quickfixgo/quickfix"
)

type Instruments struct {
	RequestID      int     `json:"request_id"`
	InstrumentName string  `json:"instrument_name"`
	SecurityDesc   string  `json:"security_desc"`
	SecurityType   string  `json:"security_type"`
	PutOrCall      string  `json:"put_or_call"`
	StrikeCurrency string  `json:"strike_currency"`
	StrikePrice    float64 `json:"strike_price"`
	Underlying     string  `json:"underlying"`
	IssueDate      string  `json:"issue_date"`
	SecurityStatus string  `json:"security_status"`
}

type MarketData struct {
	InstrumentName string  `json:"instrumentName"`
	Side           string  `json:"side"`
	Contract       string  `json:"contract"`
	Price          float64 `json:"price"`
	Amount         float64 `json:"amount"`
	Date           string  `json:"date"`
	OrderId        string  `json:"orderId"`
	SecondaryOrdId string  `json:"secondaryOrderId"`
	Status         string  `json:"status"`
	MDActionUpdate string  `json:"mdActionUpdate"`
}

var instruments []Instruments
var marketData []MarketData

// FIXApplication implements a basic quickfix.Application
type FIXApplication struct {
	SessionIDs map[string]quickfix.SessionID
	*oms.OrderManager
}

// OnLogon is ignored
func (a *FIXApplication) OnLogon(sessionID quickfix.SessionID) {}

// OnLogout is ignored
func (a *FIXApplication) OnLogout(sessionID quickfix.SessionID) {}

// ToAdmin is ignored
func (a *FIXApplication) ToAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) {
	if msg.IsMsgTypeOf(string(enum.MsgType_LOGON)) {
		_, b, _, _ := runtime.Caller(0)
		rootDir := path.Join(b, "../../")
		err := godotenv.Load(path.Join(rootDir, ".env"))
		if err != nil {
			panic(err)
		}
		msg.Body.SetString(tag.Password, os.Getenv("CLIENT_API_SECRET"))
		msg.Body.SetString(tag.Username, os.Getenv("CLIENT_API_KEY"))
	}

}

// OnCreate initialized SessionIDs
func (a *FIXApplication) OnCreate(sessionID quickfix.SessionID) {
	a.SessionIDs[sessionID.String()] = sessionID
}

// FromAdmin is ignored
func (a *FIXApplication) FromAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	return
}

// ToApp is ignored
func (a *FIXApplication) ToApp(msg *quickfix.Message, sessionID quickfix.SessionID) (err error) {
	return
}

// FromApp listens for just execution reports
func (a *FIXApplication) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, err := msg.MsgType()
	if err != nil {
		return err
	}

	switch enum.MsgType(msgType) {
	case enum.MsgType_EXECUTION_REPORT:
		fmt.Println("Execution Report")
		return a.onExecutionReport(msg, sessionID)
	case enum.MsgType_SECURITY_LIST:
		fmt.Println("Security List")
		return a.onSecurityList(msg, sessionID)
	case enum.MsgType_MARKET_DATA_SNAPSHOT_FULL_REFRESH:
		fmt.Println("Market Data Snapshot Full Refresh")
		return a.onMarketDataSnapshot(msg, sessionID)
	case enum.MsgType_MARKET_DATA_INCREMENTAL_REFRESH:
		fmt.Println("Market Data Incremental Refresh")
		return a.onMarketDataIncrementalRefresh(msg, sessionID)
	case enum.MsgType_ORDER_CANCEL_REJECT:
	}

	return quickfix.UnsupportedMessageType()
}

func (a *FIXApplication) onOrderCancelReject(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	logs.Log.Info().Str("msg", msg.String()).Msg("onOrderCancelReject")
	return nil
}

func (a *FIXApplication) onSecurityList(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.Lock()
	defer a.Unlock()
	fmt.Println("mapping security list")
	group := securitylist.NewNoRelatedSymRepeatingGroup()
	err := msg.Body.GetGroup(&group)
	if err != nil {
		fmt.Println("Error getting the group: ", err)
		return err
	}

	var secres field.SecurityResponseIDField
	msg.Body.GetField(tag.SecurityResponseID, &secres)

	var putOrCall field.PutOrCallField
	msg.Body.GetField(tag.PutOrCall, &putOrCall)

	var securityStatus field.SecurityStatusField
	msg.Body.GetField(tag.SecurityStatus, &securityStatus)

	var underlying field.UnderlyingSymbolField
	msg.Body.GetField(tag.UnderlyingSymbol, &underlying)

	var issueDate field.IssueDateField
	msg.Body.GetField(tag.IssueDate, &issueDate)

	// extract instrument name from instruments into an array of strings
	var instrumentNames []string
	for _, instrument := range instruments {
		instrumentNames = append(instrumentNames, instrument.InstrumentName)
	}

	for i := 0; i < group.Len(); i++ {
		var symbol field.SymbolField
		if err := group.Get(i).Get(&symbol); err != nil {
			return err
		}

		var securityDesc field.SecurityDescField
		if err := group.Get(i).Get(&securityDesc); err != nil {
			fmt.Println("Error getting the security desc: ", err)
		}

		var securityType field.SecurityTypeField
		if err := group.Get(i).Get(&securityType); err != nil {
			fmt.Println("Error getting the security type: ", err)
		}

		var strikePrice field.StrikePriceField
		if err := group.Get(i).Get(&strikePrice); err != nil {
			fmt.Println("Error getting the strike price: ", err)
		}
		strikePriceF, _ := strikePrice.Float64()

		var strikeCurr field.StrikeCurrencyField
		if err := group.Get(i).Get(&strikeCurr); err != nil {
			fmt.Println("Error getting the strike currency: ", err)
		}

		ins := Instruments{
			InstrumentName: symbol.String(),
			SecurityDesc:   securityDesc.String(),
			SecurityType:   securityType.String(),
			StrikePrice:    strikePriceF,
			StrikeCurrency: strikeCurr.String(),
			PutOrCall:      putOrCall.String(),
			SecurityStatus: securityStatus.String(),
			Underlying:     underlying.String(),
			IssueDate:      issueDate.String(),
		}

		if utils.ArrContains(instrumentNames, ins.InstrumentName) {
			continue
		}
		instruments = append(instruments, ins)
		instrumentNames = append(instrumentNames, ins.InstrumentName)
	}
	if instruments == nil {
		instruments = make([]Instruments, group.Len())
	}
	return nil
}

func (a *FIXApplication) onExecutionReport(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.Lock()
	defer a.Unlock()

	var clOrdID field.ClOrdIDField
	if err := msg.Body.Get(&clOrdID); err != nil {
		return err
	}

	var execType field.ExecTypeField
	if err := msg.Body.Get(&execType); err != nil {
		return err
	}

	// Order Cancel Request success, 8 = Pending Cancel
	if execType.String() == "8" {
		logs.Log.Info().Str("msg", msg.String()).Msg("Order Cancel Request success")
	}

	order, err := a.GetByClOrdID(clOrdID.String())
	if err != nil {
		log.Printf("[ERROR] err= %v", err)
		return nil
	}

	var cumQty field.CumQtyField
	if err := msg.Body.Get(&cumQty); err != nil {
		return err
	}

	var orderId field.OrderIDField
	if err := msg.Body.Get(&orderId); err != nil {
		return err
	}

	fmt.Println("OrderID: ", orderId.String())
	fmt.Println("clordid: ", clOrdID.String())
	var avgPx field.AvgPxField
	if err := msg.Body.Get(&avgPx); err != nil {
		return err
	}

	var leavesQty field.LeavesQtyField
	if err := msg.Body.Get(&leavesQty); err != nil {
		return err
	}

	order.Closed = cumQty.String()
	order.Open = leavesQty.String()
	order.AvgPx = avgPx.String()
	order.OrderID = orderId.String()
	a.Save(order)
	fmt.Println(order)
	var ordStatus field.OrdStatusField
	if err := msg.Body.Get(&ordStatus); err != nil {
		return err
	}
	fmt.Println(ordStatus.String())
	if ordStatus.String() != string(enum.OrdStatus_NEW) {
		var lastQty field.LastQtyField
		if err := msg.Body.Get(&lastQty); err != nil {
			return err
		}

		var price field.LastPxField
		if err := msg.Body.Get(&price); err != nil {
			return err
		}

		exec := new(oms.Execution)
		exec.Symbol = order.Symbol
		exec.Side = order.Side
		exec.Session = order.Session

		exec.Quantity = lastQty.String()
		exec.Price = price.String()
		_ = a.SaveExecution(exec)
	}

	return nil
}

func (a FIXApplication) GetAllSecurityList() []Instruments {
	fmt.Println("Instrument List: ", len(instruments))
	return instruments
}

func (a FIXApplication) GetMarketData() []MarketData {
	fmt.Println("Market Data: ", len(marketData))
	return marketData
}

func (a FIXApplication) onMarketDataIncrementalRefresh(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Println("onMarketDataIncrementalRefresh")
	a.Lock()
	defer a.Unlock()

	mdEntries := marketdataincrementalrefresh.NewNoMDEntriesRepeatingGroup()
	if err := msg.Body.GetGroup(&mdEntries); err != nil {
		return err
	}

	sym, _ := msg.Body.GetString(tag.Symbol)
	fmt.Println("mapping market data", mdEntries.Len())
	for i := 0; i < mdEntries.Len(); i++ {
		entry := mdEntries.Get(i)
		entryType, err := entry.GetMDEntryType()
		if err != nil {
			fmt.Println("Error getting the entry type: ", err)
		}

		entrySize, err := entry.GetMDEntrySize()
		if err != nil {
			fmt.Println("Error getting the entry size: ", err)
		}

		entryPx, err := entry.GetMDEntryPx()
		if err != nil {
			fmt.Println("Error getting the entry price: ", err)
		}

		entryDate, err := entry.GetMDEntryDate()
		if err != nil {
			fmt.Println("Error getting the entry date: ", err)
		}

		var status field.MDUpdateActionField
		if err := entry.Get(&status); err != nil {
			fmt.Println("Error getting the entry status: ", err)
		}

		var orderId field.OrderIDField
		if err := entry.Get(&orderId); err != nil {
			fmt.Println("Error getting the entry order id: ", err)
		}

		var secondaryOrderId field.SecondaryOrderIDField
		if err := entry.Get(&secondaryOrderId); err != nil {
			fmt.Println("Error getting the entry secondary order id: ", err)
		}

		var mdUpdate field.MDUpdateActionField
		if err := entry.Get(&mdUpdate); err != nil {
			fmt.Println("Error getting the entry md update action: ", err)
		}

		mdStatus := ""
		if enum.MDUpdateAction(status.String()) == enum.MDUpdateAction_NEW {
			mdStatus = "new"
		} else if enum.MDUpdateAction(status.String()) == enum.MDUpdateAction_CHANGE {
			mdStatus = "change"
		} else if enum.MDUpdateAction(status.String()) == enum.MDUpdateAction_DELETE {
			mdStatus = "delete"
		}

		marketData = append(marketData, MarketData{
			InstrumentName: sym,
			Side:           string(entryType),
			Amount:         entrySize.InexactFloat64(),
			Price:          entryPx.InexactFloat64(),
			Date:           entryDate,
			Status:         mdStatus,
			OrderId:        orderId.String(),
			SecondaryOrdId: secondaryOrderId.String(),
			MDActionUpdate: mdUpdate.String(),
		})

	}

	return nil
}

func (a FIXApplication) onMarketDataSnapshot(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.Lock()
	defer a.Unlock()

	mdEntries := marketdatasnapshotfullrefresh.NewNoMDEntriesRepeatingGroup()
	if err := msg.Body.GetGroup(&mdEntries); err != nil {
		return err
	}

	sym, _ := msg.Body.GetString(tag.Symbol)
	fmt.Println("mapping market data", mdEntries.Len())
	for i := 0; i < mdEntries.Len(); i++ {
		entry := mdEntries.Get(i)
		entryType, err := entry.GetMDEntryType()
		if err != nil {
			fmt.Println("Error getting the entry type: ", err)
		}

		entrySize, err := entry.GetMDEntrySize()
		if err != nil {
			fmt.Println("Error getting the entry size: ", err)
		}

		entryPx, err := entry.GetMDEntryPx()
		if err != nil {
			fmt.Println("Error getting the entry price: ", err)
		}

		entryDate, err := entry.GetMDEntryDate()
		if err != nil {
			fmt.Println("Error getting the entry date: ", err)
		}

		var status field.StatusTextField
		if err := entry.Get(&status); err != nil {
			fmt.Println("Error getting the entry status: ", err)
		}

		var orderId field.OrderIDField
		if err := entry.Get(&orderId); err != nil {
			fmt.Println("Error getting the entry order id: ", err)
		}

		var secondaryOrderId field.SecondaryOrderIDField
		if err := entry.Get(&secondaryOrderId); err != nil {
			fmt.Println("Error getting the entry secondary order id: ", err)
		}

		marketData = append(marketData, MarketData{
			InstrumentName: sym,
			Side:           string(entryType),
			Amount:         entrySize.InexactFloat64(),
			Price:          entryPx.InexactFloat64(),
			Date:           entryDate,
			Status:         status.String(),
			OrderId:        orderId.String(),
			SecondaryOrdId: secondaryOrderId.String(),
			MDActionUpdate: "new",
		})

	}

	return nil
}

func appendMarketData(market MarketData) []MarketData {
	duplicate := false
	fmt.Println("appending market data", market.Side)
	if len(marketData) == 0 {
		marketData = append(marketData, market)
	} else {
		for _, m := range marketData {
			if m.InstrumentName == market.InstrumentName && m.Side == market.Side && m.Price == market.Price {
				duplicate = true
			}
			fmt.Println("duplicate", duplicate)
			if !duplicate {
				marketData = append(marketData, market)
			}
		}
	}

	return marketData
}
