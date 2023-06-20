package middleware

import (
	"context"
	"fmt"
	"gateway/pkg/protocol"
	"gateway/pkg/ws"
	"time"

	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// Middleware is the middleware for gin.
type MiddlewareWs struct {
	Limiter *limiter.Limiter
}

// NewMiddleware return a new instance of a gin middleware.
func RateLimiterWs(input interface{}, c *ws.Client) *protocol.RPCResponseMessage {
	fmt.Println("RateLimiterWs ===>>>")
	middleware := &MiddlewareWs{
		Limiter: &limiter.Limiter{
			Store: memory.NewStore(),
			Rate: limiter.Rate{
				Period: 1 * time.Minute,
				Limit:  5,
			},
		},
	}

	return middleware.Handle(c)
}

// Handle gin request.
func (middleware *MiddlewareWs) Handle(c *ws.Client) *protocol.RPCResponseMessage {
	// key := c.RemoteAddr().String()
	context, err := middleware.Limiter.Get(context.Background(), "key")
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
			Message: "Too Many Requests",
			Data: protocol.ReasonMessage{
				Reason: "TOO_MANY_REQUESTS",
			},
			Code: 10028,
		},
		Testnet: true,
	}

	return &m
}
