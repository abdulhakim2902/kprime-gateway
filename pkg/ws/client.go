package ws

import (
	"gateway/pkg/utils"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// const TradeChannel = "trades"
const OrderbookChannel = "order_book"

// const OrderChannel = "orders"

type Client struct {
	*websocket.Conn
	mu   sync.Mutex
	send chan WebsocketResponseMessage
}

type SendMessageParams struct {
	UserID        string `json:"user_id"`
	ID            uint64 `json:"id"`
	RequestedTime uint64 `json:"requested_time"`
}

type WebsocketMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	ID      uint64      `json:"id"`
	Params  interface{} `json:"params"`
}

type WebsocketResponseMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      uint64      `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	UsIn    uint64      `json:"usIn,omitempty"`
	UsOut   uint64      `json:"usOut,omitempty"`
	UsDiff  uint64      `json:"usDiff,omitempty"`
	Testnet bool        `json:"testnet,omitempty"`
}

type WebsocketEvent struct {
	Type    string      `json:"type"`
	Hash    string      `json:"hash,omitempty"`
	Payload interface{} `json:"payload"`
}

var unsubscribeHandlers map[*Client][]func(*Client)
var subscriptionMutex sync.Mutex
var registerRequestRpcIdsMutex sync.Mutex

// To Validate rps id-s and return usIn,usOut,usDiff
var orderRequestRpcIDS map[string]uint64

func (c *Client) RegisterRequestRpcIDS(id string, requestedTime uint64) (bool, string) {
	registerRequestRpcIdsMutex.Lock()
	defer registerRequestRpcIdsMutex.Unlock()

	if len(id) == 0 || id[0:1] == "-" {
		return false, "Request id is required"
	}

	if orderRequestRpcIDS == nil {
		orderRequestRpcIDS = make(map[string]uint64)
	}

	if orderRequestRpcIDS[id] == 0 {
		// logger.Info("Registering a new order connection")
		orderRequestRpcIDS[id] = requestedTime
		return true, ""
	}

	return false, "Duplicated request id"
}

func NewClient(c *websocket.Conn) *Client {
	subscriptionMutex.Lock()
	defer subscriptionMutex.Unlock()
	conn := &Client{Conn: c, mu: sync.Mutex{}, send: make(chan WebsocketResponseMessage)}

	if unsubscribeHandlers == nil {
		unsubscribeHandlers = make(map[*Client][]func(*Client))
	}

	if unsubscribeHandlers[conn] == nil {
		unsubscribeHandlers[conn] = make([]func(*Client), 0)
	}

	return conn
}

// SendMessage constructs the message with proper structure to be sent over websocket
func (c *Client) SendMessage(payload interface{}, params SendMessageParams) {
	var m WebsocketResponseMessage
	m = WebsocketResponseMessage{
		Result:  payload,
		JSONRPC: "2.0",
		ID:      params.ID,
		Testnet: true,
	}

	if params.RequestedTime > 0 {
		m.UsIn = params.RequestedTime
		m.UsOut = uint64(time.Now().UnixMicro())
		m.UsDiff = m.UsOut - m.UsIn
	} else if params.ID > 0 {
		// Read requested time
		ID := utils.GetKeyFromIdUserID(params.ID, params.UserID)
		requestedTime := orderRequestRpcIDS[ID]
		// Return times
		if requestedTime > 0 {
			m.UsIn = requestedTime
			m.UsOut = uint64(time.Now().UnixMicro())
			m.UsDiff = m.UsOut - m.UsIn

			// Remove saved time
			delete(orderRequestRpcIDS, ID)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.send <- m
}

// SendMessageSubcription constructs the message with proper structure to be sent over websocket for subcription
func (c *Client) SendMessageSubcription(payload interface{}, method string, params SendMessageParams) {
	m := WebsocketResponseMessage{
		Params:  payload,
		JSONRPC: "2.0",
		Method:  method,
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.send <- m
}

func (c *Client) closeConnection() {
	subscriptionMutex.Lock()
	defer subscriptionMutex.Unlock()

	for _, unsub := range unsubscribeHandlers[c] {
		go unsub(c)
	}

	c.Close()
}

func (c *Client) SendOrderErrorMessage(err error) {
	p := map[string]interface{}{
		"message": err.Error(),
	}

	e := WebsocketEvent{
		Type:    "ERROR",
		Payload: p,
	}

	m := WebsocketResponseMessage{
		Result:  e,
		JSONRPC: "2.0",
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.send <- m
}
