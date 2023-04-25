package service

import "gateway/pkg/ws"

type IwsEngineService interface {
	Subscribe(c *ws.Client, instrument string, params ...uint64)
	Unsubscribe(c *ws.Client)
}
