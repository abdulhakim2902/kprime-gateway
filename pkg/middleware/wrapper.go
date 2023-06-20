package middleware

import (
	"gateway/pkg/protocol"
	"gateway/pkg/ws"
)

type WsHandler func(input interface{}, c *ws.Client) *protocol.RPCResponseMessage

func MiddlewaresWrapper(next WsHandler, middlewares ...WsHandler) func(input interface{}, c *ws.Client) {
	for _, middleware := range middlewares {

		rpcResponse := middleware()
		if rpcResponse != nil {
			return
		}
	}

	return next
}
