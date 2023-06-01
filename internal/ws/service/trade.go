package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	_types "gateway/internal/orderbook/types"
	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"

	"gateway/internal/deribit/model"
	_deribitModel "gateway/internal/deribit/model"
	_engineType "gateway/internal/engine/types"

	"git.devucc.name/dependencies/utilities/types"
	"github.com/Shopify/sarama"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

var userTradesMutex sync.RWMutex
var userTrades = make(map[string][]*_deribitModel.DeribitGetUserTradesByInstruments)

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

func (svc wsTradeService) HandleConsumeUserTrades(msg *sarama.ConsumerMessage) {
	var data _engineType.EngineResponse
	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(data.Matches.Trades) > 0 {
		_instrument := data.Matches.Trades[0].Underlying + "-" + data.Matches.Trades[0].ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Matches.Trades[0].StrikePrice) + "-" + string(data.Matches.Trades[0].Contracts[0])

		var tradeId []interface{}
		var userId []interface{}
		keys := make(map[interface{}]bool)
		keysUser := make(map[interface{}]bool)
		for _, trade := range data.Matches.Trades {
			if _, ok := keys[trade.ID]; !ok {
				keys[trade.ID] = true
				tradeId = append(tradeId, trade.ID)
				if _, ok := keysUser[trade.Taker.UserID]; !ok {
					keysUser[trade.Taker.UserID] = true
					userId = append(userId, trade.Taker.UserID)
				}
				if _, ok := keysUser[trade.Maker.UserID]; !ok {
					keysUser[trade.Maker.UserID] = true
					userId = append(userId, trade.Maker.UserID)
				}
			}
		}
		trades, err := svc.repo.FindUserTradesById(
			_instrument,
			userId,
			tradeId,
		)
		if err != nil {
			return
		}
		if len(trades.Trades) > 0 {
			for _, id := range userId {
				mapIndex := fmt.Sprintf("%s-%s", _instrument, id)
				if _, ok := userTrades[mapIndex]; !ok {
					userTradesMutex.Lock()
					userTrades[mapIndex] = trades.Trades
					userTradesMutex.Unlock()
					go svc.HandleConsumeUserTrades100ms(_instrument, id.(string))
				} else {
					userTradesMutex.Lock()
					userTrades[mapIndex] = append(userTrades[mapIndex], trades.Trades...)
					userTradesMutex.Unlock()
				}
				// broadcast to user id
				broadcastId := fmt.Sprintf("%s.%s.%s-%s", "user", "trades", _instrument, id)

				params := _types.QuoteResponse{
					Channel: fmt.Sprintf("user.trades.%s.raw", _instrument),
					Data:    trades.Trades,
				}
				method := "subscription"
				ws.GetTradeSocket().BroadcastMessageTrade(broadcastId, method, params)
			}
		}
		return
	} else {
		return
	}
}

func (svc wsTradeService) HandleConsumeUserTrades100ms(instrument string, userId string) {
	mapIndex := fmt.Sprintf("%s-%s", instrument, userId)
	ticker := time.NewTicker(100 * time.Millisecond)

	// Creating channel
	tickerChan := make(chan bool)
	go func() {
		for {
			select {
			case <-tickerChan:
				return
			case <-ticker.C:
				// if there is no change no need to broadcast
				userTradesMutex.RLock()
				trades := userTrades[mapIndex]
				userTradesMutex.RUnlock()
				if len(trades) > 0 {
					broadcastId := fmt.Sprintf("%s.%s.%s-%s-100ms", "user", "trades", instrument, userId)
					params := _types.QuoteResponse{
						Channel: fmt.Sprintf("user.trades.%s.100ms", instrument),
						Data:    trades,
					}
					method := "subscription"
					ws.GetTradeSocket().BroadcastMessageTrade(broadcastId, method, params)
					userTradesMutex.Lock()
					userTrades[mapIndex] = []*_deribitModel.DeribitGetUserTradesByInstruments{}
					userTradesMutex.Unlock()
				}
			}
		}
	}()
}

func (svc wsTradeService) HandleConsumeInstrumentTrades(msg *sarama.ConsumerMessage) {
	var data _engineType.EngineResponse
	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(data.Matches.Trades) > 0 {
		_instrument := data.Matches.Trades[0].Underlying + "-" + data.Matches.Trades[0].ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Matches.Trades[0].StrikePrice) + "-" + string(data.Matches.Trades[0].Contracts[0])

		var tradeId []interface{}
		keys := make(map[interface{}]bool)
		for _, trade := range data.Matches.Trades {
			if _, ok := keys[trade.ID]; !ok {
				keys[trade.ID] = true
				tradeId = append(tradeId, trade.ID)
			}
		}

		trades, err := svc.repo.FindTradesByInstrument(
			_instrument,
			tradeId,
		)
		if err != nil {
			return
		}

		if len(trades.Trades) > 0 {
			mapIndex := _instrument
			if _, ok := userTrades[mapIndex]; !ok {
				userTradesMutex.Lock()
				userTrades[mapIndex] = trades.Trades
				userTradesMutex.Unlock()
				go svc.HandleConsumeInstrumentTrades100ms(_instrument)
			} else {
				userTradesMutex.Lock()
				userTrades[mapIndex] = append(userTrades[mapIndex], trades.Trades...)
				userTradesMutex.Unlock()
			}
			// broadcast to user id
			broadcastId := fmt.Sprintf("%s.%s", "trades", _instrument)

			params := _types.QuoteResponse{
				Channel: fmt.Sprintf("trades.%s.raw", _instrument),
				Data:    trades.Trades,
			}
			method := "subscription"
			ws.GetTradeSocket().BroadcastMessageTrade(broadcastId, method, params)
		}
		return
	} else {
		return
	}
}

func (svc wsTradeService) HandleConsumeInstrumentTrades100ms(instrument string) {
	mapIndex := instrument
	ticker := time.NewTicker(100 * time.Millisecond)

	// Creating channel
	tickerChan := make(chan bool)
	go func() {
		for {
			select {
			case <-tickerChan:
				return
			case <-ticker.C:
				// if there is no change no need to broadcast
				userTradesMutex.RLock()
				trades := userTrades[mapIndex]
				userTradesMutex.RUnlock()
				if len(trades) > 0 {
					broadcastId := fmt.Sprintf("%s.%s-100ms", "trades", instrument)
					params := _types.QuoteResponse{
						Channel: fmt.Sprintf("trades.%s.100ms", instrument),
						Data:    trades,
					}
					method := "subscription"
					ws.GetTradeSocket().BroadcastMessageTrade(broadcastId, method, params)
					userTradesMutex.Lock()
					userTrades[mapIndex] = []*_deribitModel.DeribitGetUserTradesByInstruments{}
					userTradesMutex.Unlock()
				}
			}
		}
	}()
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

func (svc wsTradeService) SubscribeUserTrades(c *ws.Client, channel string, userId string) {
	socket := ws.GetTradeSocket()

	key := strings.Split(channel, ".")

	// Subscribe

	var id string
	if key[3] == "100ms" {
		id = fmt.Sprintf("%s.%s.%s-%s-100ms", key[0], key[1], key[2], userId)
	} else {
		id = fmt.Sprintf("%s.%s.%s-%s", key[0], key[1], key[2], userId)
	}
	err := socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))
}

func (svc wsTradeService) SubscribeTrades(c *ws.Client, channel string) {
	socket := ws.GetTradeSocket()

	key := strings.Split(channel, ".")

	// Subscribe

	var id string
	if key[2] == "100ms" {
		id = fmt.Sprintf("%s.%s-100ms", key[0], key[1])
	} else {
		id = fmt.Sprintf("%s.%s", key[0], key[1])
	}

	err := socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))
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

func (svc *wsTradeService) GetUserTradesByOrder(ctx context.Context, userId string, InstrumentName string, data model.DeribitGetUserTradesByOrderRequest) []model.DeribitGetUserTradesByOrderResponse {
	_getFilterUserTradesByOrder := svc.repo.FilterUserTradesByOrder(userId, InstrumentName, data)

	bsonResponse := _getFilterUserTradesByOrder

	_getUserTradesByOrder := []model.DeribitGetUserTradesByOrderValue{}

	for _, doc := range bsonResponse {
		bsonData, err := bson.Marshal(doc)
		if err != nil {
			log.Println("Error marshaling BSON to JSON:", err)
			continue
		}

		var jsonDoc map[string]interface{}
		err = bson.Unmarshal(bsonData, &jsonDoc)
		if err != nil {
			log.Println("Error unmarshaling BSON to JSON:", err)
			continue
		}

		underlying := jsonDoc["underlying"].(string)
		expiryDate := jsonDoc["expiryDate"].(string)
		strikePrice := jsonDoc["strikePrice"].(float64)
		contracts := jsonDoc["contracts"].(string)

		switch contracts {
		case "CALL":
			contracts = "C"
		case "PUT":
			contracts = "P"
		}

		resultData := model.DeribitGetUserTradesByOrderValue{
			Amount:         jsonDoc["amount"].(float64),
			Direction:      jsonDoc["side"].(string),
			InstrumentName: fmt.Sprintf("%s-%s-%d-%s", underlying, expiryDate, int64(strikePrice), contracts),
			Price:          jsonDoc["price"].(float64),
			Timestamp:      time.Now().UnixNano() / int64(time.Millisecond),
			TradeId:        jsonDoc["tradeSequence"].(int32),
			Api:            true,
			IndexPrice:     jsonDoc["indexPrice"].(float64),
			TickDirection:  jsonDoc["tickDirection"].(int32),
			TradeSeq:       jsonDoc["tradeSequence"].(int32),
			CreatedAt:      jsonDoc["createdAt"].(primitive.DateTime).Time(),
		}

		_getUserTradesByOrder = append(_getUserTradesByOrder, resultData)
	}

	results := model.DeribitGetUserTradesByOrderResponse{
		Trades: _getUserTradesByOrder,
	}

	return []model.DeribitGetUserTradesByOrderResponse{results}
}
