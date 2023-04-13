package service

import (
	"encoding/json"
	"fmt"
	"gateway/internal/orderbook/types"
	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"

	daoType "gateway/internal/repositories/types"

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

func (svc wsOrderService) initialData(key string) ([]*daoType.Order, error) {
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

// Key can be all or user Id. So channel: ORDER.all or ORDER.user123
func (svc wsOrderService) Subscribe(c *ws.Client, key string) {
	socket := ws.GetOrderSocket()

	// Get initial data from the redis
	res, err := svc.redis.GetValue("ORDER-" + key)

	// Handle the initial data
	if res == "" || err != nil {
		initData, err := svc.initialData(key)
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
	var initData []daoType.Order
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

func (svc wsOrderService) Unsubscribe(c *ws.Client) {
	socket := ws.GetOrderSocket()
	socket.Unsubscribe(c)
}
