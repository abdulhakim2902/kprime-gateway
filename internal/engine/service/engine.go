package service

import (
	"encoding/json"
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"

	"gateway/internal/engine/types"

	"gateway/pkg/redis"
	"gateway/pkg/ws"
)

type engineHandler struct {
	redis *redis.RedisConnectionPool
}

func NewEngineHandler(r *gin.Engine, redis *redis.RedisConnectionPool) IEngineService {
	return &engineHandler{redis}

}
func (svc engineHandler) HandleConsume(msg *sarama.ConsumerMessage) {
	var data types.EngineResponse

	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	//convert instrument name
	_instrument := data.Order.Underlying + "-" + data.Order.ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Order.StrikePrice) + "-" + string(data.Order.Contracts[0])

	// Save to redis
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
		return
	}

	//get redis

	//new variable with append data

	svc.redis.Set("ENGINE-"+_instrument, string(jsonBytes))

	// Broadcast
	ws.GetEngineSocket().BroadcastMessage(_instrument, data)
}
