package middleware

import (
	"gateway/pkg/protocol"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ulule/limiter/v3"
)

// Middleware is the middleware for gin.
type Middleware struct {
	Limiter *limiter.Limiter
}

// NewMiddleware return a new instance of a gin middleware.
func RateLimiter(limiter *limiter.Limiter) gin.HandlerFunc {
	middleware := &Middleware{
		Limiter: limiter,
	}

	return func(ctx *gin.Context) {
		middleware.Handle(ctx)
	}
}

// Handle gin request.
func (middleware *Middleware) Handle(c *gin.Context) {
	key := c.ClientIP()
	context, err := middleware.Limiter.Get(c, key)
	if err != nil {
		c.Abort()
		return
	}

	c.Header("X-RateLimit-Limit", strconv.FormatInt(context.Limit, 10))
	c.Header("X-RateLimit-Remaining", strconv.FormatInt(context.Remaining, 10))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(context.Reset, 10))

	if context.Reached {
		middleware.HandleLimitReached(c)
		return
	}

	c.Next()
}

func (middleware *Middleware) HandleLimitReached(c *gin.Context) {
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

	c.JSON(429, m)
	c.Abort()
	return
}
