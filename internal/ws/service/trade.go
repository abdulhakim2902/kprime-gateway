package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	deribitModel "gateway/internal/deribit/model"
	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"

	"gateway/internal/engine/types"

	"github.com/Shopify/sarama"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type wsTradeService struct {
	redis     *redis.RedisConnectionPool
	repo      *repositories.TradeRepository
	orderRepo *repositories.OrderRepository
}

func NewWSTradeService(
	redis *redis.RedisConnectionPool,
	repo *repositories.TradeRepository,
	orderRepo *repositories.OrderRepository,
) IwsTradeService {
	return &wsTradeService{redis, repo, orderRepo}
}

func (svc wsTradeService) initialData() ([]*types.Trade, error) {
	trades, err := svc.repo.Find(nil, nil, 0, -1)
	return trades, err
}

func (svc wsTradeService) HandleConsume(msg *sarama.ConsumerMessage) {
	var trade types.Trade
	if err := json.Unmarshal(msg.Value, &trade); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	var newTrade []types.Trade
	var optType string
	switch trade.Contracts {
	case types.CALL:
		optType = "C"
	case types.PUT:
		optType = "P"
	}
	instrument := fmt.Sprintf("TRADE-%s-%s-%d-%s",
		trade.Underlying,
		trade.ExpiryDate,
		int(trade.StrikePrice),
		optType,
	)

	// Get existing data from the redis
	res, err := svc.redis.GetValue(instrument)
	if res != "" || err == nil {
		err = json.Unmarshal([]byte(res), &newTrade)
		if err != nil {
			fmt.Println("Error parsing JSON:", err)
			return
		}
	}

	newTrade = append(newTrade, trade)

	data, err := json.Marshal(newTrade)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Save trade to redis
	svc.redis.Set(instrument, string(data))

	// Broadcast to users
	ws.GetTradeSocket().BroadcastMessage(instrument, newTrade)

	// Handle All
	// Get existing data from the redis
	res, err = svc.redis.GetValue("TRADE-all")
	if res != "" || err == nil {
		err = json.Unmarshal([]byte(res), &newTrade)
		if err != nil {
			fmt.Println("Error parsing JSON:", err)
			return
		}
	}
	newTrade = append(newTrade, trade)

	data, err = json.Marshal(newTrade)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Save trade to redis
	svc.redis.Set("TRADE-all", string(data))

	// Broadcast to users
	ws.GetTradeSocket().BroadcastMessage("all", newTrade)

}

func (svc wsTradeService) Subscribe(c *ws.Client, instrument string) {
	socket := ws.GetTradeSocket()

	// Get initial data from the redis
	res, err := svc.redis.GetValue("TRADE-" + instrument)

	// Handle the initial data
	if res == "" || err != nil {
		initData, err := svc.initialData()
		if err != nil {
			socket.SendInitMessage(c, &types.ErrorMessage{
				Error: err.Error(),
			})
			return
		}
		jsonBytes, err := json.Marshal(initData)
		if err != nil {
			fmt.Println(err)
			return
		}
		svc.redis.Set("TRADE-"+instrument, string(jsonBytes))

		res, _ = svc.redis.GetValue("TRADE-" + instrument)
	}

	// Subscribe
	id := instrument
	err = socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// JSON Parse
	var initData []types.Trade
	err = json.Unmarshal([]byte(res), &initData)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))

	// Send initial data from the redis
	socket.SendInitMessage(c, initData)
}

func (svc wsTradeService) Unsubscribe(c *ws.Client) {
	socket := ws.GetTradeSocket()
	socket.Unsubscribe(c)
}

func (svc wsTradeService) GetUserTradesByInstrument(
	ctx context.Context,
	userId string,
	request deribitModel.DeribitGetUserTradesByInstrumentsRequest,
) []deribitModel.DeribitGetUserTradesByInstrumentsResponse {

	_string := request.InstrumentName
	substring := strings.Split(_string, "-")

	_strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		fmt.Println(err)
	}
	_underlying := substring[0]
	_expiryDate := strings.ToUpper(substring[1])

	filter := map[string]interface{}{
		"underlying":  _underlying,
		"expiryDate":  _expiryDate,
		"strikePrice": _strikePrice,
	}
	sort := map[string]interface{}{}

	trades, err := svc.repo.Find(filter, sort, 0, -1)
	if err != nil {
		fmt.Println(err)
	}

	out := make([]deribitModel.DeribitGetUserTradesByInstrumentsResponse, 0)

	for _, trade := range trades {
		item := deribitModel.DeribitGetUserTradesByInstrumentsResponse{
			TradeId:   trade.ID.Hex(),
			Amount:    trade.Amount,
			Direction: string(trade.Side),
			Price:     trade.Price,
		}

		var orderId primitive.ObjectID
		if trade.TakerID == userId {
			orderId = trade.TakerOrderID
		} else {
			orderId = trade.MakerOrderID
		}

		item.OrderId = orderId.Hex()
		order, err := svc.orderRepo.FindByID(orderId)
		if err != nil {
			fmt.Println(err)
		}

		// Order.Type based on the order_id
		item.OrderType = string(order.Type)
		// Status of Order based on the order_id
		item.State = string(order.Status)

		out = append(out, item)
	}

	return out
}
