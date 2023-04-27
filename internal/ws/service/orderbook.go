package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	deribitModel "gateway/internal/deribit/model"
	"gateway/internal/orderbook/types"
	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"

	"go.mongodb.org/mongo-driver/bson"
)

type wsOrderbookService struct {
	redis           *redis.RedisConnectionPool
	orderRepository *repositories.OrderRepository
}

func NewwsOrderbookService(redis *redis.RedisConnectionPool, orderRepository *repositories.OrderRepository) IwsOrderbookService {
	return &wsOrderbookService{redis, orderRepository}
}

func (svc wsOrderbookService) Subscribe(c *ws.Client, instrument string) {
	socket := ws.GetOrderBookSocket()

	// Get initial data from the redis
	res, err := svc.redis.GetValue("ORDERBOOK-" + instrument)
	if res == "" || err != nil {
		socket.SendInitMessage(c, &types.Message{
			Instrument: instrument,
		})
	}

	// Subscribe
	id := instrument
	err = socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))

	// If redis is null, then stop here
	if res == "" || err != nil {
		return
	}

	// JSON Parse
	var initData types.Message
	err = json.Unmarshal([]byte(res), &initData)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Send initial data from the redis
	socket.SendInitMessage(c, initData)
}

func (svc wsOrderbookService) Unsubscribe(c *ws.Client) {
	socket := ws.GetOrderBookSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderbookService) GetOrderBook(ctx context.Context, data deribitModel.DeribitGetOrderBookRequest) deribitModel.DeribitGetOrderBookResponse {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")

	_strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		panic(err)
	}
	_underlying := substring[0]
	_expiryDate := strings.ToUpper(substring[1])

	_order := types.GetOrderBook{
		InstrumentName: _string,
		Underlying:     _underlying,
		ExpiryDate:     _expiryDate,
		StrikePrice:    _strikePrice,
	}

	// TODO query to orders collections
	_getOrderBook := svc._getOrderBook(_order)

	//count best Ask
	maxAskPrice := 0.0
	maxAskAmount := 0.0
	var maxAskItem []*types.WsOrder
	for _, item := range _getOrderBook.Asks {
		if item.Price > maxAskPrice {
			maxAskPrice = item.Price
			maxAskItem = []*types.WsOrder{item}
			maxAskAmount = item.Amount
		} else if item.Price == maxAskPrice {
			maxAskItem = append(maxAskItem, item)
			maxAskAmount += item.Amount
		}
	}

	//count best Bid
	maxBidPrice := _getOrderBook.Bids[0].Price
	maxBidAmount := 0.0
	var maxBidItem []*types.WsOrder
	for _, item := range _getOrderBook.Bids {
		if item.Price < maxBidPrice {
			maxBidPrice = item.Price
			maxBidItem = []*types.WsOrder{item}
			maxBidAmount = item.Amount
		} else if item.Price == maxBidPrice {
			maxBidItem = append(maxBidItem, item)
			maxBidAmount += item.Amount
		}
	}

	results := deribitModel.DeribitGetOrderBookResponse{
		InstrumentName: _getOrderBook.InstrumentName,
		Bids:           _getOrderBook.Bids,
		Asks:           _getOrderBook.Asks,
		BestAskPrice:   maxAskPrice,
		BestAskAmount:  maxAskAmount,
		BestBidPrice:   maxBidPrice,
		BestBidAmount:  maxBidAmount,
	}

	return results
}

func (svc wsOrderbookService) _getOrderBook(o types.GetOrderBook) *types.Orderbook {
	bidsQuery := []bson.M{
		{
			"$match": bson.M{
				"status":      bson.M{"$in": []types.OrderStatus{types.OPEN, types.PARTIAL_FILLED}},
				"underlying":  o.Underlying,
				"strikePrice": o.StrikePrice,
				"expiryDate":  o.ExpiryDate,
				"side":        types.BUY,
			},
		},
		{
			"$sort": bson.M{
				"price":     1,
				"createdAt": 1,
			},
		},
	}

	asksQuery := []bson.M{
		{
			"$match": bson.M{
				"status":      bson.M{"$in": []types.OrderStatus{types.OPEN, types.PARTIAL_FILLED}},
				"underlying":  o.Underlying,
				"strikePrice": o.StrikePrice,
				"expiryDate":  o.ExpiryDate,
				"side":        types.SELL,
			},
		},
		{
			"$sort": bson.M{
				"price":     -1,
				"createdAt": 1,
			},
		},
	}

	asks := svc.orderRepository.WsAggregate(asksQuery)
	bids := svc.orderRepository.WsAggregate(bidsQuery)

	orderbooks := &types.Orderbook{
		InstrumentName: o.InstrumentName,
		Asks:           asks,
		Bids:           bids,
	}

	return orderbooks
}
