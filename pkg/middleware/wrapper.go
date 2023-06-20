package middleware

import (
	"gateway/pkg/protocol"
	"gateway/pkg/ws"
)

type WsHandlerFunc func(interface{}, *ws.Client)

type WsMiddewareFunc func(interface{}, *ws.Client) *protocol.RPCResponseMessage

func MiddlewaresWrapper(handler WsHandlerFunc, middlewares ...WsMiddewareFunc) WsHandlerFunc {
	return func(i interface{}, c *ws.Client) {
		for _, middleware := range middlewares {
			if res := middleware(i, c); res != nil {
				return
			}
		}

		handler(i, c)
	}
}
