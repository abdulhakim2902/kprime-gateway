package service

import (
	"gateway/pkg/ws"

	"github.com/gin-gonic/gin"
)

type wsOrderbookService struct {
	//
}

func NewwsOrderbookService() IwsOrderbookService {
	return &wsOrderbookService{}
}

func (svc wsOrderbookService) Subscribe(c *ws.Client, instrument string) {
	socket := ws.GetOrderBookSocket()

	// TODO: Read from REDIS
	ob := gin.H{"todo": "read from redis"}

	id := instrument

	err := socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))
	socket.SendInitMessage(c, ob)
}

func (svc wsOrderbookService) Unsubscribe(c *ws.Client) {
	socket := ws.GetOrderBookSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderbookService) Consume() {
	// This function will be a handler from Kafka consumer for orderbook
	// We can broadcast using this method
	// ws.GetOrderBookSocket().BroadcastMessage(id, map[string]interface{}{
	// 	"pairName": orders[0].PairName,
	// 	"bids":     bids,
	// 	"asks":     asks,
	// })
}
