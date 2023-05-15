package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix43/securitylistrequest"
	"github.com/quickfixgo/fix44/marketdatarequest"
	"github.com/quickfixgo/tag"
	"github.com/quickfixgo/traderui/basic"
	"github.com/quickfixgo/traderui/oms"
	"github.com/quickfixgo/traderui/secmaster"

	"github.com/quickfixgo/quickfix"
)

type fixFactory interface {
	NewOrderSingle(ord oms.Order) (msg quickfix.Messagable, err error)
	OrderCancelRequest(ord oms.Order, clOrdID string) (msg quickfix.Messagable, err error)
	OrderCancelReplaceRequest(ord oms.Order, clOrdID string) (msg quickfix.Messagable, err error)
	SecurityDefinitionRequest(req secmaster.SecurityDefinitionRequest) (msg quickfix.Messagable, err error)
}

type fixApp interface {
	GetAllSecurityList() []basic.Instruments
	GetMarketData() []basic.MarketData
}

type tradeClient struct {
	SessionIDs map[string]quickfix.SessionID
	fixFactory
	fixApp
	*oms.OrderManager
}

func newTradeClient(factory fixFactory, idGen oms.ClOrdIDGenerator, app fixApp) *tradeClient {
	tc := &tradeClient{
		SessionIDs:   make(map[string]quickfix.SessionID),
		fixFactory:   factory,
		OrderManager: oms.NewOrderManager(idGen),
		fixApp:       app,
	}

	return tc
}

func (c tradeClient) SessionsAsJSON() (string, error) {
	fmt.Println("sessionsasjson")
	sessionIDs := make([]string, 0, len(c.SessionIDs))

	for s := range c.SessionIDs {
		sessionIDs = append(sessionIDs, s)
	}

	b, err := json.Marshal(sessionIDs)
	return string(b), err
}

func (c tradeClient) OrdersAsJSON() (string, error) {
	c.RLock()
	defer c.RUnlock()

	b, err := json.Marshal(c.GetAll())
	return string(b), err
}

func (c tradeClient) ExecutionsAsJSON() (string, error) {
	c.RLock()
	defer c.RUnlock()

	b, err := json.Marshal(c.GetAllExecutions())
	return string(b), err
}

func (c tradeClient) SecurityListAsJSON() (string, error) {
	fmt.Println("seclist as json")

	c.RLock()
	defer c.RUnlock()

	b, err := json.Marshal(c.GetAllSecurityList())
	return string(b), err
}

func (c tradeClient) MarketDataAsJSON() (string, error) {
	fmt.Println("marketdata as json")

	c.RLock()
	defer c.RUnlock()

	b, err := json.Marshal(c.GetMarketData())
	return string(b), err
}

func (c tradeClient) traderView(w http.ResponseWriter, r *http.Request) {
	var templates = template.Must(template.New("traderui").ParseFiles("tmpl/index.html"))
	if err := templates.ExecuteTemplate(w, "index.html", c); err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c tradeClient) fetchRequestedOrder(r *http.Request) (*oms.Order, error) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		panic(err)
	}

	return c.Get(id)
}

func (c tradeClient) fetchRequestedExecution(r *http.Request) (*oms.Execution, error) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		panic(err)
	}

	return c.GetExecution(id)
}

func (c tradeClient) getOrder(w http.ResponseWriter, r *http.Request) {
	c.RLock()
	defer c.RUnlock()

	order, err := c.fetchRequestedOrder(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	c.writeOrderJSON(w, order)
}

func (c tradeClient) writeOrderJSON(w http.ResponseWriter, order *oms.Order) {
	outgoingJSON, err := json.Marshal(order)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(outgoingJSON))
}

func (c tradeClient) getExecution(w http.ResponseWriter, r *http.Request) {

	c.RLock()
	defer c.RUnlock()

	exec, err := c.fetchRequestedExecution(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	outgoingJSON, err := json.Marshal(exec)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(outgoingJSON))
}

func (c tradeClient) getInstruments(w http.ResponseWriter, r *http.Request) {

	c.RLock()
	defer c.RUnlock()

	b, err := json.Marshal(c.GetAllSecurityList())
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	fmt.Println(string(b))
	if string(b) == "null" {
		b = []byte("[no instruments]")
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(b))
}

func (c tradeClient) getMarketData(w http.ResponseWriter, r *http.Request) {
	fmt.Println("get market data")
	c.RLock()
	defer c.RUnlock()

	b, err := json.Marshal(c.GetMarketData())
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	fmt.Println(string(b))
	if string(b) == "null" {
		b = []byte("[no market data]")
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(b))
}

func (c tradeClient) updateOrder(w http.ResponseWriter, r *http.Request) {
	c.Lock()
	defer c.Unlock()

	order, err := c.fetchRequestedOrder(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	clOrdID := c.AssignNextClOrdID(order)
	msg, err := c.OrderCancelReplaceRequest(*order, clOrdID)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println("updateOrder", order.OrderID)
	qty, _ := strconv.ParseInt(order.Quantity, 10, 16)
	msg.ToMessage().Body.SetField(quickfix.Tag(44), quickfix.FIXDecimal{order.PriceDecimal, 2})
	msg.ToMessage().Body.SetInt(quickfix.Tag(38), int(qty))
	msg.ToMessage().Body.SetString(quickfix.Tag(37), order.OrderID)
	err = quickfix.SendToTarget(msg, order.SessionID)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	c.writeOrderJSON(w, order)
}

func (c tradeClient) deleteOrder(w http.ResponseWriter, r *http.Request) {
	c.Lock()
	defer c.Unlock()

	order, err := c.fetchRequestedOrder(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	fmt.Println("deletingggg", order.OrderID)
	clOrdID := c.AssignNextClOrdID(order)
	msg, err := c.OrderCancelRequest(*order, clOrdID)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	msg.ToMessage().Body.SetString(quickfix.Tag(448), "2") // clientid
	msg.ToMessage().Body.SetString(quickfix.Tag(37), order.OrderID)
	err = quickfix.SendToTarget(msg, order.SessionID)
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	c.writeOrderJSON(w, order)
}

func (c tradeClient) getOrders(w http.ResponseWriter, r *http.Request) {
	outgoingJSON, err := c.OrdersAsJSON()
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, outgoingJSON)
}

func (c tradeClient) getExecutions(w http.ResponseWriter, r *http.Request) {
	outgoingJSON, err := c.ExecutionsAsJSON()
	if err != nil {
		log.Printf("[ERROR] err = %+v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, outgoingJSON)
}

func (c tradeClient) newSecurityDefintionRequest(w http.ResponseWriter, r *http.Request) {
	var secDefRequest secmaster.SecurityDefinitionRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&secDefRequest)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("secDefRequest = %+v\n", secDefRequest)

	if sessionID, ok := c.SessionIDs[secDefRequest.Session]; ok {
		secDefRequest.SessionID = sessionID
	} else {
		log.Println("[ERROR] Invalid SessionID")
		http.Error(w, "Invalid SessionID", http.StatusBadRequest)
		return
	}

	msg, err := c.fixFactory.SecurityDefinitionRequest(secDefRequest)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = quickfix.SendToTarget(msg, secDefRequest.SessionID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c tradeClient) newOrder(w http.ResponseWriter, r *http.Request) {
	var order oms.Order
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&order)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if sessionID, ok := c.SessionIDs[order.Session]; ok {
		order.SessionID = sessionID
	} else {
		log.Println("[ERROR] Invalid SessionID")
		http.Error(w, "Invalid SessionID", http.StatusBadRequest)
		return
	}

	if err = order.Init(); err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.Lock()
	_ = c.OrderManager.Save(&order)
	c.Unlock()

	fmt.Println(order.Session)
	msg, err := c.NewOrderSingle(order)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	msg.ToMessage().Body.SetString(tag.PartyID, order.PartyID)
	msg.ToMessage().Body.SetString(tag.ClOrdID, order.ClOrdID)
	msg.ToMessage().Body.SetField(quickfix.Tag(44), quickfix.FIXDecimal{order.PriceDecimal, 2})
	err = quickfix.SendToTarget(msg, order.SessionID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c tradeClient) onSecurityListRequest(w http.ResponseWriter, r *http.Request) {
	var secDefRequest secmaster.SecurityDefinitionRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&secDefRequest)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("secDefRequest = %+v\n", secDefRequest)

	if sessionID, ok := c.SessionIDs[secDefRequest.Session]; ok {
		secDefRequest.SessionID = sessionID
	} else {
		log.Println("[ERROR] Invalid SessionID")
		http.Error(w, "Invalid SessionID", http.StatusBadRequest)
		return
	}
	newMsg := securitylistrequest.New(
		field.NewSecurityReqID("1"),
		field.NewSecurityListRequestType(enum.SecurityListRequestType_SYMBOL),
	)
	fmt.Println("request subs type", secDefRequest.SubscriptionRequestType)
	subsType, _ := strconv.Atoi(secDefRequest.SubscriptionRequestType)
	newMsg.SetInt(tag.SubscriptionRequestType, subsType)
	newMsg.SetString(tag.Currency, secDefRequest.Symbol) // btc / all
	fmt.Println("requesting security list")
	err = quickfix.SendToTarget(newMsg, secDefRequest.SessionID)
	if err != nil {
		fmt.Println("Error sending security list request,", err)
	}
}

func (c tradeClient) onMarketDataRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Println("onMarketDataRequest")
	var mktDataRequest secmaster.MarketDataRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&mktDataRequest)
	fmt.Println("mktDataRequest", mktDataRequest)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if sessionID, ok := c.SessionIDs[mktDataRequest.Session]; ok {
		mktDataRequest.SessionID = sessionID
	} else {
		log.Println("[ERROR] Invalid SessionID")
		http.Error(w, "Invalid SessionID", http.StatusBadRequest)
		return
	}

	msg := marketdatarequest.New(
		field.NewMDReqID("1"),
		field.NewSubscriptionRequestType(enum.SubscriptionRequestType(mktDataRequest.SubscriptionRequestType)),
		field.NewMarketDepth(0),
	)

	mdEntryGrp := marketdatarequest.NewNoMDEntryTypesRepeatingGroup()

	fmt.Println("mktDataRequest.Bid", mktDataRequest.Bid)
	if mktDataRequest.Bid == "Bid" {
		fmt.Println("adding bid")
		mdEntryGrp.Add().SetMDEntryType(enum.MDEntryType_BID)
	}

	fmt.Println("mktDataRequest.Ask", mktDataRequest.Ask)
	if mktDataRequest.Ask == "Ask" {
		fmt.Println("adding ask")
		mdEntryGrp.Add().SetMDEntryType(enum.MDEntryType_OFFER)
	}

	fmt.Println("mktDataRequest.Trade", mktDataRequest.Trade)
	if mktDataRequest.Trade == "Trade" {
		fmt.Println("adding trade")
		mdEntryGrp.Add().SetMDEntryType(enum.MDEntryType_TRADE)
	}

	fmt.Println("mdEntryGrp", mdEntryGrp, mdEntryGrp.Len())
	msg.SetNoMDEntryTypes(mdEntryGrp)

	mdReqGrp := marketdatarequest.NewNoRelatedSymRepeatingGroup()

	symbols := strings.Split(mktDataRequest.Symbol, ",")

	for _, symbol := range symbols {
		mdReqGrp.Add().SetString(tag.Symbol, symbol)
	}

	msg.SetNoRelatedSym(mdReqGrp)
	fmt.Println(msg.Message.String())
	err = quickfix.SendToTarget(msg, mktDataRequest.SessionID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	flag.Parse()

	cfgFileName := path.Join("config", "tradeclient.cfg")
	if flag.NArg() > 0 {
		cfgFileName = flag.Arg(0)
	}

	cfg, err := os.Open(cfgFileName)
	if err != nil {
		fmt.Printf("Error opening %v, %v\n", cfgFileName, err)
		return
	}

	appSettings, err := quickfix.ParseSettings(cfg)
	if err != nil {
		fmt.Println("Error reading cfg,", err)
		return
	}

	logFactory := NewFancyLog()

	var fixApp quickfix.Application
	app := newTradeClient(basic.FIXFactory{}, new(basic.ClOrdIDGenerator), basic.FIXApplication{})
	fixApp = &basic.FIXApplication{
		SessionIDs:   app.SessionIDs,
		OrderManager: app.OrderManager,
	}

	initiator, err := quickfix.NewInitiator(fixApp, quickfix.NewMemoryStoreFactory(), appSettings, logFactory)
	if err != nil {
		log.Fatalf("Unable to create Initiator: %s\n", err)
	}

	if err = initiator.Start(); err != nil {
		log.Fatal(err)
	}
	defer initiator.Stop()

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/orders", app.newOrder).Methods("POST")
	router.HandleFunc("/orders", app.getOrders).Methods("GET")
	router.HandleFunc("/orders/{id:[0-9]+}", app.updateOrder).Methods("PUT")
	router.HandleFunc("/orders/{id:[0-9]+}", app.getOrder).Methods("GET")
	router.HandleFunc("/orders/{id:[0-9]+}", app.deleteOrder).Methods("DELETE")

	router.HandleFunc("/instruments", app.getInstruments).Methods("GET")
	router.HandleFunc("/marketdata", app.getMarketData).Methods("GET")

	router.HandleFunc("/executions", app.getExecutions).Methods("GET")
	router.HandleFunc("/executions/{id:[0-9]+}", app.getExecution).Methods("GET")

	router.HandleFunc("/securitydefinitionrequest", app.onSecurityListRequest).Methods("POST")
	router.HandleFunc("/marketdatarequest", app.onMarketDataRequest).Methods("POST")

	router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	router.HandleFunc("/", app.traderView)

	log.Fatal(http.ListenAndServe(":8081", router))
}
