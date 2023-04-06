package service

import "gateway/pkg/ws"

type IwsOrderbookService interface {
	Subscribe(c *ws.Client, instrument string)
	Unsubscribe(c *ws.Client)
}
