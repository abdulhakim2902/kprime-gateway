package basic

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"

	"github.com/joho/godotenv"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
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

var instruments []Instruments

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
	}

	return quickfix.UnsupportedMessageType()
}

func (a *FIXApplication) onSecurityList(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.Lock()
	defer a.Unlock()
	fmt.Println("mapping security list")
	group := securitylist.NewNoRelatedSymRepeatingGroup()
	fmt.Println("a")
	err := msg.Body.GetGroup(&group)
	fmt.Println("b")
	if err != nil {
		fmt.Println("Error getting the group: ", err)
		return err
	}
	fmt.Println("c", group.Len())

	var secres field.SecurityResponseIDField
	msg.Body.GetField(tag.SecurityResponseID, &secres)
	fmt.Println("secres", secres)

	var putOrCall field.PutOrCallField
	msg.Body.GetField(tag.PutOrCall, &putOrCall)

	var securityStatus field.SecurityStatusField
	msg.Body.GetField(tag.SecurityStatus, &securityStatus)

	var underlying field.UnderlyingSymbolField
	msg.Body.GetField(tag.UnderlyingSymbol, &underlying)

	var issueDate field.IssueDateField
	msg.Body.GetField(tag.IssueDate, &issueDate)

	for i := 0; i < group.Len(); i++ {
		var symbol field.SymbolField
		if err := group.Get(i).Get(&symbol); err != nil {
			return err
		}

		var securityDesc field.SecurityDescField
		if err := group.Get(i).Get(&securityDesc); err != nil {
			return err
		}

		var securityType field.SecurityTypeField
		if err := group.Get(i).Get(&securityType); err != nil {
			return err
		}

		var strikePrice field.StrikePriceField
		if err := group.Get(i).Get(&strikePrice); err != nil {
			return err
		}
		strikePriceF, _ := strikePrice.Float64()

		var strikeCurr field.StrikeCurrencyField
		if err := group.Get(i).Get(&strikeCurr); err != nil {
			return err
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
		instruments = append(instruments, ins)
	}
	if instruments == nil {
		instruments = make([]Instruments, group.Len())
	}
	fmt.Println("Instrument List: ", instruments)
	return nil
}

func (a *FIXApplication) onExecutionReport(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	a.Lock()
	defer a.Unlock()

	var clOrdID field.ClOrdIDField
	if err := msg.Body.Get(&clOrdID); err != nil {
		return err
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
	if msg.Body.Has(tag.LastShares) {
		var lastShares field.LastSharesField
		if err := msg.Body.Get(&lastShares); err != nil {
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

		exec.Quantity = lastShares.String()
		exec.Price = price.String()
		_ = a.SaveExecution(exec)
	}

	return nil
}

func (a FIXApplication) GetAllSecurityList() []Instruments {
	fmt.Println("Instrument List: ", instruments)
	return instruments
}
