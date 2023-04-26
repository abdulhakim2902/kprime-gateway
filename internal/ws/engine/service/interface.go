package service

import "gateway/pkg/ws"

type IwsEngineService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
}
