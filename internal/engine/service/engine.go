package service

import (
	"encoding/json"
	"fmt"
	"gateway/pkg/protocol"
	"gateway/pkg/utils"
	"strconv"
	"time"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/types"
	"git.devucc.name/dependencies/utilities/types/validation_reason"
	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"

	_engineType "gateway/internal/engine/types"
	_ordermatch "gateway/internal/fix-acceptor"
	_orderbookTypes "gateway/internal/orderbook/types"
	wsService "gateway/internal/ws/service"

	orderType "git.devucc.name/dependencies/utilities/models/order"

	"gateway/internal/repositories"

	"gateway/pkg/redis"
	"gateway/pkg/ws"
)

type engineHandler struct {
	redis     *redis.RedisConnectionPool
	tradeRepo *repositories.TradeRepository
	wsOBSvc   wsService.IwsOrderbookService
}

func NewEngineHandler(
	r *gin.Engine,
	redis *redis.RedisConnectionPool,
	tradeRepo *repositories.TradeRepository,
	wsOBSvc wsService.IwsOrderbookService,
) IEngineService {
	return &engineHandler{redis, tradeRepo, wsOBSvc}

}
func (svc engineHandler) HandleConsume(msg *sarama.ConsumerMessage) {
	var data _engineType.EngineResponse
	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		reason := validation_reason.PARSE_ERROR
		data.Validation = reason

		svc.PublishValidation(data)
		return
	}

	if data.Status == types.ORDER_REJECTED {
		svc.PublishValidation(data)
		return
	}

	//convert instrument name
	_instrument := data.Matches.TakerOrder.Underlying + "-" + data.Matches.TakerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Matches.TakerOrder.StrikePrice) + "-" + string(data.Matches.TakerOrder.Contracts[0])

	// Publish actions
	switch data.Status {
	case types.ORDER_ADDED, types.ORDER_PARTIALLY_FILLED, types.ORDER_FILLED, types.ORDER_CANCELLED, types.ORDER_AMENDED:
		svc.PublishOrder(data)
	}

	//check date if more than 3 days ago, remove from redis
	checkDateToRemoveRedis(data.Matches.TakerOrder.ExpiryDate, _instrument, svc)

	//init redisDataArray variable
	redisDataArray := []_engineType.EngineResponse{}

	//get redis
	redisData, err := svc.redis.GetValue("ENGINE-" + _instrument)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
	}

	//create new variable with array of object and append to redisDataArray
	if redisData != "" {
		var _redisDataArray []_engineType.EngineResponse
		err = json.Unmarshal([]byte(redisData), &_redisDataArray)
		if err != nil {
			reason := validation_reason.PARSE_ERROR
			data.Validation = reason

			svc.PublishValidation(data)
			return
		}

		redisDataArray = append(_redisDataArray, data)
	} else {
		redisDataArray = append(redisDataArray, data)
	}

	//convert redisDataArray to json
	jsonBytes, err := json.Marshal(redisDataArray)
	if err != nil {
		reason := validation_reason.PARSE_ERROR
		data.Validation = reason

		svc.PublishValidation(data)
		return
	}
	_ordermatch.OnMatchingOrder(data)

	//pass to fix gateway

	//save to redis
	svc.redis.Set("ENGINE-"+_instrument, string(jsonBytes))

	//broadcast to websocket
	ws.GetEngineSocket().BroadcastMessage(_instrument, data)
}

func (svc engineHandler) HandleConsumeQuote(msg *sarama.ConsumerMessage) {
	var data _engineType.EngineResponse
	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	if data.Matches == nil && len(data.Matches.TakerOrder.Contracts) > 0 {
		return
	}

	//convert instrument name
	_instrument := data.Matches.TakerOrder.Underlying + "-" + data.Matches.TakerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Matches.TakerOrder.StrikePrice) + "-" + string(data.Matches.TakerOrder.Contracts[0])

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: _instrument,
		Underlying:     data.Matches.TakerOrder.Underlying,
		ExpiryDate:     data.Matches.TakerOrder.ExpiryDate,
		StrikePrice:    data.Matches.TakerOrder.StrikePrice,
	}

	initData, _ := svc.wsOBSvc.GetDataQuote(_order)

	//convert redisDataArray to json
	jsonBytes, err := json.Marshal(initData)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	//save to redis
	svc.redis.Set("QUOTE-"+_instrument, string(jsonBytes))

	//broadcast to websocket

	params := _orderbookTypes.QuoteResponse{
		Channel: fmt.Sprintf("quote.%s", _instrument),
		Data:    initData,
	}
	method := "subscription"
	ws.GetQuoteSocket().BroadcastMessage(_instrument, method, params)
}

func (svc engineHandler) HandleConsumeQuoteCancel(msg *sarama.ConsumerMessage) {
	var data orderType.CancelledOrder

	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, order := range data.Data {
		//convert instrument name
		_instrument := order.Underlying + "-" + order.ExpiryDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0])

		_order := _orderbookTypes.GetOrderBook{
			InstrumentName: _instrument,
			Underlying:     order.Underlying,
			ExpiryDate:     order.ExpiryDate,
			StrikePrice:    order.StrikePrice,
		}

		initData, _ := svc.wsOBSvc.GetDataQuote(_order)

		//convert redisDataArray to json
		jsonBytes, err := json.Marshal(initData)
		if err != nil {
			logs.Log.Error().Err(err).Msg("")
			return
		}

		//save to redis
		svc.redis.Set("QUOTE-"+_instrument, string(jsonBytes))

		//broadcast to websocket

		params := _orderbookTypes.QuoteResponse{
			Channel: fmt.Sprintf("quote.%s", _instrument),
			Data:    initData,
		}
		method := "subscription"
		ws.GetQuoteSocket().BroadcastMessage(_instrument, method, params)
	}
}

func checkDateToRemoveRedis(_expiryDate string, _instrument string, svc engineHandler) {
	layout := "02Jan06"
	dateString := _expiryDate
	date, err := time.Parse(layout, dateString)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}
	formattedDate := date.Format("2006-01-02")

	// Parse the given date string into a time.Time value
	dateStr := formattedDate
	_date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
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

func (svc engineHandler) PublishOrder(data _engineType.EngineResponse) {
	instrumentName := data.Matches.TakerOrder.Underlying + "-" + data.Matches.TakerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Matches.TakerOrder.StrikePrice) + "-" + string(data.Matches.TakerOrder.Contracts[0])

	tradePriceAvg, err := svc.tradeRepo.GetPriceAvg(
		data.Matches.TakerOrder.Underlying,
		data.Matches.TakerOrder.ExpiryDate,
		string(data.Matches.TakerOrder.Contracts),
		data.Matches.TakerOrder.StrikePrice,
	)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
		return
	}

	order := _engineType.BuySellEditCancelOrder{
		OrderState:          types.OrderStatus(data.Matches.TakerOrder.Status),
		Usd:                 data.Matches.TakerOrder.Price,
		FilledAmount:        data.Matches.TakerOrder.FilledAmount,
		InstrumentName:      instrumentName,
		Direction:           types.Side(data.Matches.TakerOrder.Side),
		LastUpdateTimestamp: utils.MakeTimestamp(data.Matches.TakerOrder.UpdatedAt),
		Price:               data.Matches.TakerOrder.Price,
		Replaced:            len(data.Matches.TakerOrder.Amendments) > 0,
		Amount:              data.Matches.TakerOrder.Amount,
		OrderId:             data.Matches.TakerOrder.ID,
		OrderType:           types.Type(data.Matches.TakerOrder.Type),
		TimeInForce:         types.TimeInForce(data.Matches.TakerOrder.TimeInForce),
		CreationTimestamp:   utils.MakeTimestamp(data.Matches.TakerOrder.CreatedAt),
		Label:               data.Matches.TakerOrder.Label,
		Api:                 true,
		CancelReason:        data.Matches.TakerOrder.CancelledReason.String(),
		AveragePrice:        tradePriceAvg,
		MaxShow:             data.Matches.TakerOrder.MaxShow,
		PostOnly:            data.Matches.TakerOrder.PostOnly,
		ReduceOnly:          data.Matches.TakerOrder.ReduceOnly,
	}

	ID, _ := strconv.ParseUint(data.Matches.TakerOrder.ClOrdID, 0, 64)
	connKey := utils.GetKeyFromIdUserID(ID, data.Matches.TakerOrder.UserID)

	switch data.Status {
	case types.ORDER_CANCELLED:
		protocol.SendSuccessMsg(connKey, _engineType.CancelResponse{
			Order: order,
		})
		break
	default:
		trades := []_engineType.BuySellEditTrade{}
		if data.Matches != nil && data.Matches.Trades != nil && len(data.Matches.Trades) > 0 {
			for _, element := range data.Matches.Trades {
				trades = append(trades, _engineType.BuySellEditTrade{
					Advanced:       "usd",
					Amount:         element.Amount,
					Direction:      element.Side,
					InstrumentName: instrumentName,
					OrderId:        data.Matches.TakerOrder.ID,
					OrderType:      types.Type(data.Matches.TakerOrder.Type),
					Price:          element.Price,
					State:          element.Status,
					Timestamp:      utils.MakeTimestamp(element.CreatedAt),
					TradeId:        element.ID,
					Api:            true,
					Label:          data.Matches.TakerOrder.Label,
					TickDirection:  element.TickDirection,
					TradeSequence:  element.TradeSequence,
					IndexPrice:     element.IndexPrice,
				})
			}
		}

		protocol.SendSuccessMsg(connKey, _engineType.BuySellEditResponse{
			Order:  order,
			Trades: trades,
		})
		break
	}

}

func (svc engineHandler) PublishValidation(data _engineType.EngineResponse) {
	ID, _ := strconv.ParseUint(data.Matches.TakerOrder.ClOrdID, 0, 64)
	connKey := utils.GetKeyFromIdUserID(ID, data.Matches.TakerOrder.UserID)

	protocol.SendValidationMsg(connKey, data.Validation, nil)
}
