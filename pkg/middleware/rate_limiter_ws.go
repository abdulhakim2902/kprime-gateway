package middleware

import (
	"context"
	"gateway/pkg/protocol"
	"gateway/pkg/ws"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
	"github.com/ulule/limiter/v3"
)

// Middleware is the middleware for gin.
type MiddlewareWs struct {
	Limiter *limiter.Limiter
}

var WSLimiter *limiter.Limiter

func SetupWSLimiter(wsLimiter *limiter.Limiter) {
	rateLimiter := limiter.New(wsLimiter.Store, wsLimiter.Rate)
	WSLimiter = rateLimiter
}

// NewMiddleware return a new instance of a gin middleware.
func RateLimiterWs(input interface{}, c *ws.Client) *protocol.RPCResponseMessage {
	middleware := &MiddlewareWs{
		Limiter: WSLimiter,
	}

	return middleware.Handle(c)
}

// Handle gin request.
func (middleware *MiddlewareWs) Handle(c *ws.Client) *protocol.RPCResponseMessage {
	key := c.RemoteAddr().String()
	context, err := middleware.Limiter.Get(context.TODO(), key)
	if err != nil {
		return &protocol.RPCResponseMessage{
			Error: &protocol.ErrorMessage{
				Message: err.Error(),
			},
		}
	}

	if context.Reached {
		return middleware.HandleLimitReachedWs(c)
	}

	return nil
}

func (middleware *MiddlewareWs) HandleLimitReachedWs(c *ws.Client) *protocol.RPCResponseMessage {
	m := protocol.RPCResponseMessage{
		JSONRPC: "2.0",
		Result:  nil,
		Error: &protocol.ErrorMessage{
			Message: validation_reason.TOO_MANY_REQUESTS.String(),
			Data: protocol.ReasonMessage{
				Reason: validation_reason.TOO_MANY_REQUESTS.String(),
			},
			Code: 10028,
		},
		Testnet: true,
	}
	msg := ws.WebsocketResponseMessage{
		JSONRPC: m.JSONRPC,
		ID:      m.ID,
		Method:  m.Method,
		Testnet: m.Testnet,
		UsIn:    m.UsIn,
		UsOut:   m.UsOut,
		UsDiff:  m.UsDiff,
		Error:   m.Error,
	}
	c.Send(msg)
	return &m
}
