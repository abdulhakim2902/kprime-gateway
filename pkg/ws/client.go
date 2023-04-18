package ws

import (
	"sync"

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
	ID      string      `json:"id"`
	Params  interface{} `json:"params"`
}

type WebsocketResponseMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Result  interface{} `json:"result"`
}

type WebsocketEvent struct {
	Type    string      `json:"type"`
	Hash    string      `json:"hash,omitempty"`
	Payload interface{} `json:"payload"`
}

var unsubscribeHandlers map[*Client][]func(*Client)
var subscriptionMutex sync.Mutex

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
func (c *Client) SendMessage(payload interface{}, params ...string) {
	// e := WebsocketEvent{
	// 	Type:    msgType,
	// 	Payload: payload,
	// }
	responseID := ""
	// Read first parameter if it exists from optional params
	if len(params) > 0 {
		responseID = params[0]
	}

	m := WebsocketResponseMessage{
		Result:  payload,
		JSONRPC: "2.0",
		ID:      responseID,
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
		ID:      "",
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.send <- m
}
