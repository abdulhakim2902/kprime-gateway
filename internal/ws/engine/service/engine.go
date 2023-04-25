package service

import (
	"encoding/json"
	"gateway/internal/engine/types"
	"gateway/pkg/redis"
	"gateway/pkg/ws"
)

type wsEngineService struct {
	redis *redis.RedisConnectionPool
}

func NewwsEngineService(redis *redis.RedisConnectionPool) IwsEngineService {
	return &wsEngineService{redis}
}

func (svc wsEngineService) Subscribe(c *ws.Client, instrument string, params ...uint64) {
	socket := ws.GetEngineSocket()
	// Get initial data from the redis
	res, err := svc.redis.GetValue("ENGINE-" + instrument)
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
		socket.SendErrorMessage(c, msg, params[0])
		return
	}

	// JSON Parse
	var initData types.Message
	err = json.Unmarshal([]byte(res), &initData)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg, params[0])
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))

	// Send initial data from the redis
	socket.SendInitMessage(c, initData, params[0])
}

func (svc wsEngineService) Unsubscribe(c *ws.Client) {
	socket := ws.GetEngineSocket()
	socket.Unsubscribe(c)
}
