package service

import "gateway/pkg/ws"

type IwsEngineService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
	SubscribeHeartbeat(c *ws.Client, connKey string, interval int)
	AddHeartbeat(c *ws.Client)
}
