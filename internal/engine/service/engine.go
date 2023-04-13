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
	svc.redis.Set("ENGINE-"+data.Instrument, string(jsonBytes))

	// Broadcast
	ws.GetEngineSocket().BroadcastMessage(data.Instrument, data)
}
