package service

import (
	"encoding/json"
	"fmt"
	"time"

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

	//check date if more than 3 days ago, remove from redis
	checkDateToRemoveRedis(data.Order.ExpiryDate, _instrument, svc)

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

func checkDateToRemoveRedis(_expiryDate string, _instrument string, svc engineHandler) {
	layout := "02Jan06"
	dateString := _expiryDate
	date, err := time.Parse(layout, dateString)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	formattedDate := date.Format("2006-01-02")

	// Parse the given date string into a time.Time value
	dateStr := formattedDate
	_date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		fmt.Println("Error parsing date:", err)
		return
	}

	// Get the current time
	currentTime := time.Now()

	// Subtract 3 days from the current time
	threeDaysAgo := currentTime.AddDate(0, 0, -3)

	// Compare the dates
	if _date.Before(threeDaysAgo) {
		// Remove from redis
		removeRedis := svc.redis.Del("ENGINE-" + _instrument)
		fmt.Println("removeRedis", removeRedis)
		return
	}
}
