package service

import (
	"context"
	"encoding/json"
	"fmt"
	deribitModel "gateway/internal/deribit/model"
	"strings"
	"sync"
	"time"

	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"

	engineType "gateway/internal/engine/types"
	_types "gateway/internal/orderbook/types"

	"github.com/Shopify/sarama"
	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/kafka"

	"go.mongodb.org/mongo-driver/bson"
)

type wsOrderService struct {
	redis *redis.RedisConnectionPool
	repo  *repositories.OrderRepository
}

var userOrdersMutex sync.RWMutex
var userOrders = make(map[string][]deribitModel.DeribitGetOpenOrdersByInstrumentResponse)

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
	if err != nil {
		fmt.Println(err)
		return
	}
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

	if data.Matches == nil && len(data.Matches.TakerOrder.Contracts) > 0 {
		return
	}
	_instrument := data.Matches.TakerOrder.Underlying + "-" + data.Matches.TakerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Matches.TakerOrder.StrikePrice) + "-" + string(data.Matches.TakerOrder.Contracts[0])

	var orderId []interface{}
	var userId []interface{}
	keys := make(map[interface{}]bool)
	if len(data.Matches.MakerOrders) > 0 {
		for _, order := range data.Matches.MakerOrders {
			if _, ok := keys[order.ID]; !ok {
				keys[order.ID] = true
				orderId = append(orderId, order.ID)
				userId = append(userId, order.UserID)
			}
		}
	}
	if data.Matches.TakerOrder != nil {
		order := data.Matches.TakerOrder
		if _, ok := keys[order.ID]; !ok {
			keys[order.ID] = true
			orderId = append(orderId, order.ID)
			userId = append(userId, order.UserID)
		}
	}

	orders, err := svc.repo.GetChangeOrdersByInstrument(
		_instrument,
		userId,
		orderId,
	)
	if err != nil {
		return
	}
	keys = make(map[interface{}]bool)
	for _, id := range userId {
		if _, ok := keys[id]; !ok {
			keys[id] = true
			for _, order := range orders {
				if id == order.UserId.Hex() {
					mapIndex := fmt.Sprintf("%s-%s", _instrument, id)
					if _, ok := userOrders[mapIndex]; !ok {
						order100ms := []deribitModel.DeribitGetOpenOrdersByInstrumentResponse{order}
						userOrdersMutex.Lock()
						userOrders[mapIndex] = order100ms
						userOrdersMutex.Unlock()
						go svc.HandleConsumeUserOrder100ms(_instrument, id.(string))
					} else {
						userOrdersMutex.Lock()
						userOrders[mapIndex] = append(userOrders[mapIndex], order)
						userOrdersMutex.Unlock()
					}
					// broadcast to user id
					broadcastId := fmt.Sprintf("%s.%s.%s-%s", "user", "orders", _instrument, id)
					params := _types.QuoteResponse{
						Channel: fmt.Sprintf("user.orders.%s.raw", _instrument),
						Data:    order,
					}
					method := "subscription"
					ws.GetOrderSocket().BroadcastMessageOrder(broadcastId, method, params)
				}
			}
		}
	}
}

func (svc wsOrderService) HandleConsumeUserOrderCancel(msg *sarama.ConsumerMessage) {
	var data kafka.CancelledOrder

	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, order := range data.Data {
		_instrument := order.Underlying + "-" + order.ExpiryDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0])

		var orderId []interface{}
		var userId []interface{}
		orderId = append(orderId, order.ID)
		userId = append(userId, order.UserID)

		orders, err := svc.repo.GetChangeOrdersByInstrument(
			_instrument,
			userId,
			orderId,
		)
		if err != nil {
			continue
		}
		keys := make(map[interface{}]bool)
		for _, id := range userId {
			if _, ok := keys[id]; !ok {
				keys[id] = true
				for _, order := range orders {
					if id == order.UserId.Hex() {
						mapIndex := fmt.Sprintf("%s-%s", _instrument, id)
						if _, ok := userOrders[mapIndex]; !ok {
							order100ms := []deribitModel.DeribitGetOpenOrdersByInstrumentResponse{order}
							userOrdersMutex.Lock()
							userOrders[mapIndex] = order100ms
							userOrdersMutex.Unlock()
							go svc.HandleConsumeUserOrder100ms(_instrument, id.(string))
						} else {
							userOrdersMutex.Lock()
							userOrders[mapIndex] = append(userOrders[mapIndex], order)
							userOrdersMutex.Unlock()
						}
						// broadcast to user id
						broadcastId := fmt.Sprintf("%s.%s.%s-%s", "user", "orders", _instrument, id)
						params := _types.QuoteResponse{
							Channel: fmt.Sprintf("user.orders.%s.raw", _instrument),
							Data:    order,
						}
						method := "subscription"
						ws.GetOrderSocket().BroadcastMessageOrder(broadcastId, method, params)
					}
				}
			}
		}
	}
}

func (svc wsOrderService) HandleConsumeUserOrder100ms(instrument string, userId string) {
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
				userOrdersMutex.RLock()
				orders := userOrders[mapIndex]
				userOrdersMutex.RUnlock()
				if len(orders) > 0 {
					broadcastId := fmt.Sprintf("%s.%s.%s-%s-100ms", "user", "orders", instrument, userId)
					params := _types.QuoteResponse{
						Channel: fmt.Sprintf("user.orders.%s.100ms", instrument),
						Data:    orders,
					}
					method := "subscription"
					ws.GetOrderSocket().BroadcastMessageOrder(broadcastId, method, params)
					userOrdersMutex.Lock()
					userOrders[mapIndex] = []deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}
					userOrdersMutex.Unlock()
				}
			}
		}
	}()
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

// SubscribeUserOrder asyncApi
// @summary Notification user orders changes
// @description Get notifications about changes in user's orders for given instrument.
// @payload model.SubscribeChannelParameters
// @x-response model.SubscribeChannelResponse
// @contentType application/json
// @auth private
// @queue user.orders.{instrument_name}.{interval}
// @method user.orders.instrument_name.interval
// @tags private subscribe orders
// @operation subscribe
func (svc wsOrderService) SubscribeUserOrder(c *ws.Client, channel string, userId string) {
	socket := ws.GetOrderSocket()
	key := strings.Split(channel, ".")

	// Subscribe

	var id string
	if len(key) > 3 && key[3] == "100ms" {
		id = fmt.Sprintf("%s.%s.%s-%s-100ms", key[0], key[1], key[2], userId)
	} else {
		id = fmt.Sprintf("%s.%s.%s-%s", key[0], key[1], key[2], userId)
	}

	logs.Log.Info().Str("subscribe", id).Msg("")
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

	orders, err := svc.repo.GetInstruments(request.UserId, request.Currency, request.Expired)
	if err != nil {
		fmt.Println(err)
	}

	jsonBytes, err := json.Marshal(orders)
	if err != nil {
		fmt.Println(err)
	}

	var instrumentData []deribitModel.DeribitGetInstrumentsResponse
	err = json.Unmarshal(jsonBytes, &instrumentData)
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

func (svc wsOrderService) GetOrderState(ctx context.Context, userId string, request deribitModel.DeribitGetOrderStateRequest) *deribitModel.DeribitGetOrderStateResponse {
	orders, err := svc.repo.GetOrderState(
		userId,
		request.OrderId,
	)
	if err != nil {
		return nil
	}

	jsonBytes, err := json.Marshal(orders)
	if err != nil {
		fmt.Println(err)

		return nil
	}

	var orderState []deribitModel.DeribitGetOrderStateResponse
	err = json.Unmarshal([]byte(jsonBytes), &orderState)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	if len(orderState) > 0 {
		return &orderState[0]
	}

	return nil
}
