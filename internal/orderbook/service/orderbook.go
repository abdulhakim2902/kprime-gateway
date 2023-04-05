package service

import (
	"encoding/json"
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"

	"gateway/internal/orderbook/types"

	"gateway/pkg/redis"
	"gateway/pkg/ws"
)

type orderbookHandler struct {
	redis *redis.RedisConnection
}

func NewOrderbookHandler(r *gin.Engine, redis *redis.RedisConnection) IOrderbookService {
	return &orderbookHandler{redis}

}
func (svc orderbookHandler) HandleConsume(msg *sarama.ConsumerMessage) {
	var data types.Message

	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Save to redis
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
		return
	}
	svc.redis.Set("ORDERBOOK-"+data.Instrument, string(jsonBytes))

	// Broadcast
	ws.GetOrderBookSocket().BroadcastMessage(data.Instrument, data)
}
