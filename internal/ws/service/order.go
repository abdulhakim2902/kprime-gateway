package service

import (
	"context"
	"encoding/json"
	"fmt"
	deribitModel "gateway/internal/deribit/model"
	"strings"

	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"
	"strconv"

	engineType "gateway/internal/engine/types"
	_types "gateway/internal/orderbook/types"

	"github.com/Shopify/sarama"

	"go.mongodb.org/mongo-driver/bson"
)

type wsOrderService struct {
	redis *redis.RedisConnectionPool
	repo  *repositories.OrderRepository
}

func NewWSOrderService(redis *redis.RedisConnectionPool, repo *repositories.OrderRepository) IwsOrderService {
	return &wsOrderService{redis, repo}
}

func (svc wsOrderService) initialData(key string) ([]*_types.Order, error) {
	if key != "all" {
		orders, err := svc.repo.Find(bson.M{"userId": key}, nil, 0, -1)
		return orders, err
	} else {
		orders, err := svc.repo.Find(nil, nil, 0, -1)
		return orders, err
	}
}

func (svc wsOrderService) HandleConsume(msg *sarama.ConsumerMessage, userId string) {
	// Get All Orders, and Save it to the redis
	orders, err := svc.repo.Find(nil, nil, 0, -1)
	if err != nil {
		fmt.Println(err)
		return
	}
	jsonBytes, err := json.Marshal(orders)
	if err != nil {
		fmt.Println(err)
		return
	}
	svc.redis.Set("ORDER-all", string(jsonBytes))
	// Then broadcast

	ws.GetOrderSocket().BroadcastMessage("all", orders)

	// Get specific order for userId, and save it to the redis
	orders, err = svc.repo.Find(bson.M{"userId": userId}, nil, 0, -1)
	jsonBytes, err = json.Marshal(orders)
	if err != nil {
		fmt.Println(err)
		return
	}
	svc.redis.Set("ORDER-"+userId, string(jsonBytes))

	// Then broadcast
	ws.GetOrderSocket().BroadcastMessage(userId, orders)
}

func (svc wsOrderService) HandleConsumeUserOrder(msg *sarama.ConsumerMessage) {
	var data engineType.EngineResponse
	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}
	_instrument := data.Matches.TakerOrder.Underlying + "-" + data.Matches.TakerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Matches.TakerOrder.StrikePrice) + "-" + string(data.Matches.TakerOrder.Contracts[0])

	var orderId []interface{}
	var userId []interface{}
	if len(data.Matches.MakerOrders) > 0 {
		for _, order := range data.Matches.MakerOrders {
			orderId = append(orderId, order.ID)
			userId = append(userId, order.UserID)
		}
	}
	if data.Matches.TakerOrder != nil {
		order := data.Matches.TakerOrder
		orderId = append(orderId, order.ID)
		userId = append(userId, order.UserID)
	}

	orders, err := svc.repo.GetChangeOrdersByInstrument(
		_instrument,
		userId,
		orderId,
	)
	if err != nil {
		return
	}
	for _, id := range userId {
		// broadcast to user id
		broadcastId := fmt.Sprintf("%s.%s.%s-%s", "user", "orders", _instrument, id)
		ws.GetOrderSocket().BroadcastMessage(broadcastId, orders)
	}
}

// Key can be all or user Id. So channel: ORDER.all or ORDER.user123
func (svc wsOrderService) Subscribe(c *ws.Client, key string) {
	socket := ws.GetOrderSocket()

	// Get initial data from the redis
	res, err := svc.redis.GetValue("ORDER-" + key)

	// Handle the initial data
	if res == "" || err != nil {
		initData, err := svc.initialData(key)
		if err != nil {
			socket.SendInitMessage(c, &engineType.ErrorMessage{
				Error: err.Error(),
			})
			return
		}
		jsonBytes, err := json.Marshal(initData)
		if err != nil {
			fmt.Println(err)
			return
		}
		svc.redis.Set("ORDER-"+key, string(jsonBytes))

		res, _ = svc.redis.GetValue("ORDER-" + key)
	}

	// Subscribe
	id := key
	err = socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// JSON Parse
	var initData []_types.Order
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

func (svc wsOrderService) SubscribeUserOrder(c *ws.Client, channel string, userId string) {
	socket := ws.GetOrderSocket()
	key := strings.Split(channel, ".")

	// Subscribe
	id := fmt.Sprintf("%s.%s.%s-%s", key[0], key[1], key[2], userId)
	err := socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))
}

func (svc wsOrderService) Unsubscribe(c *ws.Client) {
	socket := ws.GetOrderSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderService) GetInstruments(ctx context.Context, request deribitModel.DeribitGetInstrumentsRequest) []deribitModel.DeribitGetInstrumentsResponse {

	key := "INSTRUMENTS-" + request.Currency + "" + strconv.FormatBool(request.Expired)

	// Get initial data from the redis
	res, err := svc.redis.GetValue(key)

	// Handle the initial data
	if res == "" || err != nil {
		// Get All Orders, and Save it to the redis
		orders, err := svc.repo.GetInstruments(request.Currency, request.Expired)
		if err != nil {
			fmt.Println(err)
		}

		jsonBytes, err := json.Marshal(orders)
		if err != nil {
			fmt.Println(err)
		}
		// Expire in seconds
		svc.redis.SetEx(key, string(jsonBytes), 3)

		res, _ = svc.redis.GetValue(key)
	}

	var instrumentData []deribitModel.DeribitGetInstrumentsResponse
	err = json.Unmarshal([]byte(res), &instrumentData)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return instrumentData
}

func (svc wsOrderService) GetOpenOrdersByInstrument(ctx context.Context, userId string, request deribitModel.DeribitGetOpenOrdersByInstrumentRequest) []deribitModel.DeribitGetOpenOrdersByInstrumentResponse {

	trades, err := svc.repo.GetOpenOrdersByInstrument(
		request.InstrumentName,
		request.Type,
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

	var openOrderData []deribitModel.DeribitGetOpenOrdersByInstrumentResponse
	err = json.Unmarshal([]byte(jsonBytes), &openOrderData)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return openOrderData
}

func (svc wsOrderService) GetGetOrderHistoryByInstrument(ctx context.Context, userId string, request deribitModel.DeribitGetOrderHistoryByInstrumentRequest) []deribitModel.DeribitGetOrderHistoryByInstrumentResponse {

	trades, err := svc.repo.GetOrderHistoryByInstrument(
		request.InstrumentName,
		request.Count,
		request.Offset,
		request.IncludeOld,
		request.IncludeUnfilled,
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

	var historyOrderData []deribitModel.DeribitGetOrderHistoryByInstrumentResponse
	err = json.Unmarshal([]byte(jsonBytes), &historyOrderData)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return historyOrderData
}
