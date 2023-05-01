package service

import (
	"encoding/json"
	"fmt"
	"gateway/pkg/utils"
	"strconv"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"

	"gateway/internal/engine/types"

	ordermatch "gateway/internal/fix-acceptor"
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

	// Publish actions
	switch data.Status {
	case types.ORDER_ADDED, types.ORDER_PARTIALLY_FILLED, types.ORDER_FILLED, types.ORDER_CANCELLED, types.ORDER_AMENDED:
		svc.PublishOrder(data)
	}

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

	//pass to fix gateway
	ordermatch.OnMatchingOrder(data)

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

func (svc engineHandler) PublishOrder(data types.EngineResponse) {
	instrumentName := data.Order.Underlying + "-" + data.Order.ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Order.StrikePrice) + "-" + string(data.Order.Contracts[0])
	order := types.BuySellEditCancelOrder{
		OrderState:          data.Order.Status,
		Usd:                 data.Order.Price,
		FilledAmount:        data.Order.FilledAmount,
		InstrumentName:      instrumentName,
		Direction:           data.Order.Side,
		LastUpdateTimestamp: utils.MakeTimestamp(data.Order.UpdatedAt),
		Price:               data.Order.Price,
		Replaced:            len(data.Order.Amendments) > 0,
		Amount:              data.Order.Amount,
		OrderId:             data.Order.ID,
		OrderType:           data.Order.Type,
		TimeInForce:         data.Order.TimeInForce,
		CreationTimestamp:   utils.MakeTimestamp(data.Order.CreatedAt),
	}
	userIDStr := fmt.Sprintf("%v", data.Order.UserID)
	ClOrdID := fmt.Sprintf("%v", data.Order.ClOrdID)
	ID, _ := strconv.ParseUint(ClOrdID, 0, 64)
	connectionKey := utils.GetKeyFromIdUserID(ID, userIDStr)
	switch data.Status {
	case types.ORDER_CANCELLED:
		ws.SendOrderMessage(connectionKey, types.CancelResponse{
			Order: order,
		}, ws.SendMessageParams{
			ID:     ID,
			UserID: userIDStr,
		})
	default:
		trades := []types.BuySellEditTrade{}
		if data.Matches != nil && data.Matches.Trades != nil && len(data.Matches.Trades) > 0 {
			for _, element := range data.Matches.Trades {
				trades = append(trades, types.BuySellEditTrade{
					Advanced:       "usd",
					Amount:         element.Amount,
					Direction:      element.Side,
					InstrumentName: instrumentName,
					OrderId:        data.Order.ID,
					OrderType:      data.Order.Type,
					Price:          element.Price,
					State:          element.Status,
					Timestamp:      utils.MakeTimestamp(element.CreatedAt),
					TradeId:        element.ID,
				})
			}
		}
		ws.SendOrderMessage(connectionKey, types.BuySellEditResponse{
			Order:  order,
			Trades: trades,
		}, ws.SendMessageParams{
			ID:     ID,
			UserID: userIDStr,
		})

	}

}
