package service

import (
	"context"
	"encoding/json"
	"fmt"

	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"

	_deribitModel "gateway/internal/deribit/model"
	_engineType "gateway/internal/engine/types"

	"git.devucc.name/dependencies/utilities/types"
	"github.com/Shopify/sarama"
)

type wsTradeService struct {
	redis *redis.RedisConnectionPool
	repo  *repositories.TradeRepository
}

func NewWSTradeService(
	redis *redis.RedisConnectionPool,
	repo *repositories.TradeRepository,
) IwsTradeService {
	return &wsTradeService{redis, repo}
}

func (svc wsTradeService) initialData() ([]*_engineType.Trade, error) {
	trades, err := svc.repo.Find(nil, nil, 0, -1)
	return trades, err
}

func (svc wsTradeService) HandleConsume(msg *sarama.ConsumerMessage) {
	var trade _engineType.Trade
	if err := json.Unmarshal(msg.Value, &trade); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	var newTrade []_engineType.Trade
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
			socket.SendInitMessage(c, &_engineType.ErrorMessage{
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
	var initData []_engineType.Trade
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
	request _deribitModel.DeribitGetUserTradesByInstrumentsRequest,
) *_deribitModel.DeribitGetUserTradesByInstrumentsResponse {

	trades, err := svc.repo.FindUserTradesByInstrument(
		request.InstrumentName,
		request.Sorting,
		request.Count,
		userId,
	)
	if err != nil {
		return nil
	}

	jsonBytes, err := json.Marshal(trades)
	if err != nil {
		fmt.Println(err)

		return nil
	}

	var out *_deribitModel.DeribitGetUserTradesByInstrumentsResponse
	if err = json.Unmarshal([]byte(jsonBytes), &out); err != nil {
		fmt.Println(err)

		return nil
	}

	return out
}
