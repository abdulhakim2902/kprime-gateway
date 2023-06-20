package ws

import (
	"encoding/json"
	"net/http"
	"time"

	"gateway/internal/deribit/model"
	"gateway/pkg/kafka/producer"

	"git.devucc.name/dependencies/utilities/commons/logs"
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
		logs.Log.Error().Err(err).Msg("")
		return
	}

	con := NewClient(conn)
	con.SetCloseHandler(closeHandler(con))

	go readHandler(con)
	go writeHandler(con)
}

func readHandler(c *Client) {
	defer func() {
		logs.Log.Warn().Msg("closing connection")

		if c.EnableCancel {
			PublishCancelAll(c.ConnectionKey)
		}

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
				logs.Log.Error().Err(err).Msg("")
			}

			return
		}

		if msgType != 1 {
			return
		}

		msg := WebsocketMessage{}
		if string(payload) == "PING" {
			c.SendMessage("PONG", SendMessageParams{})
		} else {
			if err := json.Unmarshal(payload, &msg); err != nil {
				logs.Log.Error().Err(err).Msg("")

				c.SendMessage(err.Error(), SendMessageParams{})
				return
			}

			if socketChannels[msg.Method] == nil {
				c.SendMessage("INVALID_CHANNEL", SendMessageParams{})
				return
			}

			if msg.ID == nil {
				c.SendMessage("INVALID_REQUEST", SendMessageParams{})
				return
			}

			go socketChannels[msg.Method](msg, c)
		}
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
				logs.Log.Error().Err(err).Msg("")
				return
			}

		case m, ok := <-c.send:
			c.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.WriteMessage(websocket.CloseMessage, []byte{})
			}

			err := c.WriteJSON(m)
			if err != nil {
				logs.Log.Error().Err(err).Msg("")
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
	// logger.Info("Registering a new unsubscribe handler")
	subscriptionMutex.Lock()
	unsubscribeHandlers[c] = append(unsubscribeHandlers[c], fn)
	subscriptionMutex.Unlock()
}

func PublishCancelAll(connkey string) {
	payload := model.DeribitCancelAllByConnectionId{
		Side:         "CANCEL_ALL_BY_CONNECTION_ID",
		ConnectionId: connkey,
	}
	out, err := json.Marshal(payload)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return
	}
	producer.KafkaProducer(string(out), "NEW_ORDER")
}
