package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 60 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// var logger = NewWebsocketLogger()

// var socketChannels map[string]func(interface{}, *Client)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ConnectionEndpoint is the the handleFunc function for websocket connections
// It handles incoming websocket messages and routes the message according to
// channel parameter in channelMessage
func ConnectionEndpoint(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("err")
		fmt.Println(err)
		// logger.Error(err)
		return
	}

	con := NewClient(conn)
	con.SetCloseHandler(closeHandler(con))

	go readHandler(con)
	go writeHandler(con)
}

func readHandler(c *Client) {
	defer func() {
		fmt.Println("Closing connection")
		// logger.Info("Closing connection")
		c.closeConnection()
	}()

	c.SetReadDeadline(time.Now().Add(pongWait))
	c.SetPongHandler(func(string) error {
		c.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		msgType, payload, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Println("err")
				fmt.Println(err)

				// logger.Error(err)
			}

			return
		}

		if msgType != 1 {
			return
		}

		msg := WebsocketMessage{}
		if err := json.Unmarshal(payload, &msg); err != nil {
			fmt.Println("err")
			fmt.Println(err)
			// logger.Error(err)
			c.SendMessage(err.Error())
			return
		}

		// logger.LogMessageIn(&msg)
		// logger.Infof("%v", msg.String())

		if socketChannels[msg.Method] == nil {
			c.SendMessage("INVALID_CHANNEL")
			return
		}

		fmt.Println("connection")
		fmt.Println(msg.Params)
		go socketChannels[msg.Method](msg, c)
	}
}

func writeHandler(c *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		// logger.Info("Closing connection")
		ticker.Stop()
		c.closeConnection()
	}()

	for {
		select {
		case <-ticker.C:
			c.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				// logger.Error(err)
				return
			}

		case m, ok := <-c.send:
			c.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.WriteMessage(websocket.CloseMessage, []byte{})
			}

			// logger.LogMessageOut(&m)
			// logger.Infof("%v", m.String())

			err := c.WriteJSON(m)
			if err != nil {
				// logger.Error(err)
				return
			}
		}
	}
}

func closeHandler(c *Client) func(code int, text string) error {
	return func(code int, text string) error {
		c.closeConnection()
		return nil
	}
}

// RegisterConnectionUnsubscribeHandler needs to be called whenever a connection subscribes to
// a new channel.
// At the time of connection closing the ConnectionUnsubscribeHandler handlers associated with
// that connection are triggered.
func RegisterConnectionUnsubscribeHandler(c *Client, fn func(*Client)) {
	subscriptionMutex.Lock()
	defer subscriptionMutex.Unlock()

	// logger.Info("Registering a new unsubscribe handler")
	unsubscribeHandlers[c] = append(unsubscribeHandlers[c], fn)
}
