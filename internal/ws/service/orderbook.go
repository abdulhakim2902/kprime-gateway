package service

import (
	"encoding/json"
	"gateway/internal/orderbook/types"
	"gateway/pkg/redis"
	"gateway/pkg/ws"
)

type wsOrderbookService struct {
	redis *redis.RedisConnectionPool
}

func NewwsOrderbookService(redis *redis.RedisConnectionPool) IwsOrderbookService {
	return &wsOrderbookService{redis}
}

func (svc wsOrderbookService) Subscribe(c *ws.Client, instrument string) {
	socket := ws.GetOrderBookSocket()

	// Get initial data from the redis
	res, err := svc.redis.GetValue("ORDERBOOK-" + instrument)
	if res == "" || err != nil {
		socket.SendInitMessage(c, &types.Message{
			Instrument: instrument,
		})
		return
	}

	// Subscribe
	id := instrument
	err = socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// JSON Parse
	var initData types.Message
	err = json.Unmarshal([]byte(res), &initData)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))

	// Send initial data from the redis
	socket.SendInitMessage(c, initData)
}

func (svc wsOrderbookService) Unsubscribe(c *ws.Client) {
	socket := ws.GetOrderBookSocket()
	socket.Unsubscribe(c)
}
