package service

import (
	"encoding/json"
	"fmt"
	"gateway/internal/engine/types"
	"gateway/pkg/redis"
	"gateway/pkg/ws"
	"time"
)

type wsEngineService struct {
	redis *redis.RedisConnectionPool
}

func NewwsEngineService(redis *redis.RedisConnectionPool) IwsEngineService {
	return &wsEngineService{redis}
}

var heartbeat = make(map[string]bool)

type Params struct {
	Type string `json:"type"`
}

func (svc wsEngineService) Subscribe(c *ws.Client, instrument string) {
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

func (svc wsEngineService) Unsubscribe(c *ws.Client) {
	socket := ws.GetEngineSocket()
	socket.Unsubscribe(c)
}

func (svc wsEngineService) SubscribeHeartbeat(c *ws.Client, connKey string, interval int) {
	socket := ws.GetEngineSocket()

	// Subscribe
	id := fmt.Sprintf("%v", &c.Conn)
	heartbeat[id] = true

	err := socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))
	// delay interval to send initial request message
	time.Sleep(time.Duration(interval) * time.Second)

	for heartbeat[id] {
		method := "heartbeat"
		params := Params{
			Type: "heartbeat",
		}
		ws.GetEngineSocket().BroadcastMessageSubcription(id, method, params)
		params = Params{
			Type: "test_request",
		}
		ws.GetEngineSocket().BroadcastMessageSubcription(id, method, params)

		heartbeat[id] = false
		// delay interval before next checking
		time.Sleep(time.Duration(interval) * time.Second)
	}
	c.Close()
}

func (svc wsEngineService) AddHeartbeat(c *ws.Client) {
	id := fmt.Sprintf("%v", &c.Conn)

	if _, ok := heartbeat[id]; ok {
		heartbeat[id] = true
	}
	return
}
