package service

import (
	"encoding/json"
	"fmt"
	"strings"

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

	// Convert BTC-28JAN22-50000.000000-C to BTC-28JAN22-50000-C"
	parts := strings.Split(data.Instrument, "-")
	// Parse the float value and convert it to an integer
	var floatValue float64
	fmt.Sscanf(parts[2], "%f", &floatValue)
	intValue := int(floatValue)
	instrument := fmt.Sprintf("%s-%s-%d-%s", parts[0], parts[1], intValue, parts[3])

	data.Instrument = instrument

	// Save to redis
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
		return
	}
	svc.redis.Set("ORDERBOOK-"+instrument, string(jsonBytes))

	// Broadcast
	ws.GetOrderBookSocket().BroadcastMessage(instrument, data)
}
