package ws

import (
	"strconv"
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

type WebsocketMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	ID      uint64      `json:"id"`
	Params  interface{} `json:"params"`
}

type WebsocketResponseMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      uint64      `json:"id"`
	Result  interface{} `json:"result"`
	UsIn    uint64      `json:"usIn"`
	UsOut   uint64      `json:"usOut"`
	UsDiff  uint64      `json:"usDiff"`
	Testnet bool        `json:"testnet"`
}

type WebsocketEvent struct {
	Type    string      `json:"type"`
	Hash    string      `json:"hash,omitempty"`
	Payload interface{} `json:"payload"`
}

var unsubscribeHandlers map[*Client][]func(*Client)
var subscriptionMutex sync.Mutex

// To Validate rps id-s and return usIn,usOut,usDiff
var orderRequestRpcIDS map[string]uint64

func (c *Client) RegisterRequestRpcIDS(id uint64, requestedTime uint64) (bool, string) {

	if id == 0 {
		return false, "Request id is required"
	}

	if orderRequestRpcIDS == nil {
		orderRequestRpcIDS = make(map[string]uint64)
	}

	if orderRequestRpcIDS[strconv.FormatUint(id, 10)] == 0 {
		// logger.Info("Registering a new order connection")
		orderRequestRpcIDS[strconv.FormatUint(id, 10)] = requestedTime
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
func (c *Client) SendMessage(payload interface{}, params ...uint64) {
	var m WebsocketResponseMessage
	responseID := uint64(0)
	// Read first parameter if it exists from optional params
	if len(params) > 0 {
		responseID = params[0]
		m = WebsocketResponseMessage{
			Result:  payload,
			JSONRPC: "2.0",
			ID:      responseID,
			Testnet: true,
		}
		// Read requested time
		ID := strconv.FormatUint(params[0], 10)
		requestedTime := orderRequestRpcIDS[ID]
		// Return times
		if requestedTime > 0 {
			m.UsIn = requestedTime
			m.UsOut = uint64(time.Now().UnixMicro())
			m.UsDiff = m.UsOut - m.UsIn

			// Remove saved time
			delete(orderRequestRpcIDS, ID)
		}
	} else {
		m = WebsocketResponseMessage{
			Result:  payload,
			JSONRPC: "2.0",
			ID:      responseID,
			Testnet: true,
		}
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
		ID:      uint64(0),
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.send <- m
}
