package service

import (
	"encoding/json"
	"fmt"
	"time"

	_types "gateway/internal/orderbook/types"
	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"

	engineType "gateway/internal/engine/types"

	"github.com/Shopify/sarama"
)

type wsRawPriceService struct {
	redis              *redis.RedisConnectionPool
	rawPriceRepository *repositories.RawPriceRepository
}

func NewWSRawPriceService(redis *redis.RedisConnectionPool, repo *repositories.RawPriceRepository) IwsRawPriceService {
	return &wsRawPriceService{redis, repo}
}

func (svc wsRawPriceService) HandleConsume(msg *sarama.ConsumerMessage) {
	// Parse data and get index name
	var data engineType.MessagePrices
	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}
	keys := make(map[interface{}]bool)
	if len(data.RawPrice) > 0 {
		for _, rawPrice := range data.RawPrice {
			if rawPrice.Metadata.Type == "index" {
				if _, ok := keys[rawPrice.Metadata.Pair]; !ok {
					index := rawPrice.Metadata.Pair
					keys[index] = true
					ts := time.Now().UnixNano() / int64(time.Millisecond)

					// broadcast every pair
					broadcastId := string(index)
					data := _types.PriceData{
						Timestamp: ts,
						Price:     rawPrice.Price,
						IndexName: string(index),
					}

					params := _types.PriceResponse{
						Channel: fmt.Sprintf("deribit_price_index.%s", index),
						Data:    data,
					}
					method := "subscription"
					ws.GetPriceSocket().BroadcastMessage(broadcastId, method, params)
				}
			}
		}
	}
}

// Key can be all or user Id. So channel: ORDER.all or ORDER.user123
func (svc wsRawPriceService) Subscribe(c *ws.Client, key string) {
	socket := ws.GetPriceSocket()

	// Subscribe
	id := key
	err := socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))
}

func (svc wsRawPriceService) Unsubscribe(c *ws.Client) {
	socket := ws.GetPriceSocket()
	socket.Unsubscribe(c)
}
