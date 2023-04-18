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

	//init redisDataArray variable
	redisDataArray := []types.EngineResponse{}

	//get redis
	redisData, err := svc.redis.GetValue("ENGINE-" + _instrument)
	if err != nil {
		fmt.Println("error get redis or redis is empty")
	}

	//create new variable with array of object and append to redisDataArray
	if redisData != "" {
		var _redisDataArray []types.EngineResponse
		err = json.Unmarshal([]byte(redisData), &_redisDataArray)
		if err != nil {
			fmt.Println("error unmarshal redisData")
			fmt.Println(err)
			return
		}

		redisDataArray = append(_redisDataArray, data)
	} else {
		redisDataArray = append(redisDataArray, data)
	}

	//convert redisDataArray to json
	jsonBytes, err := json.Marshal(redisDataArray)
	if err != nil {
		fmt.Println(err)
		return
	}

	//save to redis
	svc.redis.Set("ENGINE-"+_instrument, string(jsonBytes))

	//broadcast to websocket
	ws.GetEngineSocket().BroadcastMessage(_instrument, data)
}
