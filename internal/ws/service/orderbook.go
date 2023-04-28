package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	deribitModel "gateway/internal/deribit/model"
	_engineTypes "gateway/internal/engine/types"
	"gateway/internal/orderbook/types"
	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"

	"go.mongodb.org/mongo-driver/bson"
)

type wsOrderbookService struct {
	redis           *redis.RedisConnectionPool
	orderRepository *repositories.OrderRepository
	tradeRepository *repositories.TradeRepository
}

func NewwsOrderbookService(redis *redis.RedisConnectionPool, orderRepository *repositories.OrderRepository, tradeRepository *repositories.TradeRepository) IwsOrderbookService {
	return &wsOrderbookService{redis, orderRepository, tradeRepository}
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

	//TODO query to orders collections
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
	maxBidPrice := 0.0
	if len(_getOrderBook.Bids) > 0 {
		maxBidPrice = _getOrderBook.Bids[0].Price
	}
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

	//check state
	dateString := _expiryDate
	layout := "02Jan06"
	date, err := time.Parse(layout, dateString)
	if err != nil {
		fmt.Println("Error parsing date:", err)
	}
	currentTime := time.Now()
	oneDayAgo := currentTime.AddDate(0, 0, -1)
	_state := ""
	if date.Before(oneDayAgo) {
		_state = "closed"
	} else {
		_state = "open"
	}

	//TODO query to trades collections
	_getLastTrades := svc._getLastTrades(_order)
	_lastPrice := 0.0
	if len(_getLastTrades) > 0 {
		_lastPrice = _getLastTrades[len(_getLastTrades)-1].Price
	}

	_getHigestTrade := svc._getHighLowTrades(_order, -1)
	_hightPrice := 0.0
	if len(_getHigestTrade) > 0 {
		_hightPrice = _getHigestTrade[0].Price
	}

	_getLowestTrade := svc._getHighLowTrades(_order, 1)
	_lowestPrice := 0.0
	_volumeAmount := 0.0
	if len(_getLowestTrade) > 0 {
		_lowestPrice = _getLowestTrade[0].Price
		for _, item := range _getLowestTrade {
			_volumeAmount += item.Amount
		}
	}

	_get24HoursTrade := svc._get24HoursTrades(_order)
	_priceChange := 0.0
	if len(_get24HoursTrade) > 0 {
		_firstTrade := _get24HoursTrade[0].Price
		_lastTrade := _get24HoursTrade[len(_get24HoursTrade)-1].Price

		//calculate price change with percentage
		_priceChange = (_lastTrade - _firstTrade) / _firstTrade * 100
	}

	results := deribitModel.DeribitGetOrderBookResponse{
		InstrumentName: _getOrderBook.InstrumentName,
		Bids:           _getOrderBook.Bids,
		Asks:           _getOrderBook.Asks,
		BestAskPrice:   maxAskPrice,
		BestAskAmount:  maxAskAmount,
		BestBidPrice:   maxBidPrice,
		BestBidAmount:  maxBidAmount,
		Timestamp:      time.Now().UnixNano() / int64(time.Millisecond),
		State:          _state,
		LastPrice:      _lastPrice,
		Stats: deribitModel.OrderBookStats{
			High:        _hightPrice,
			Low:         _lowestPrice,
			PriceChange: _priceChange,
			Volume:      _volumeAmount,
		},
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

func (svc wsOrderbookService) _getLastTrades(o types.GetOrderBook) []*_engineTypes.Trade {
	tradesQuery := bson.M{
		"underlying":  o.Underlying,
		"strikePrice": o.StrikePrice,
		"expiryDate":  o.ExpiryDate,
	}
	tradesSort := bson.M{
		"createdAt": 1,
	}

	trades, err := svc.tradeRepository.Find(tradesQuery, tradesSort, 0, -1)
	if err != nil {
		panic(err)
	}

	return trades
}

func (svc wsOrderbookService) _getHighLowTrades(o types.GetOrderBook, t int) []*_engineTypes.Trade {
	currentTime := time.Now()
	oneDayAgo := currentTime.AddDate(0, 0, -1)

	tradesQuery := bson.M{
		"underlying":  o.Underlying,
		"strikePrice": o.StrikePrice,
		"expiryDate":  o.ExpiryDate,
		"createdAt": bson.M{
			"$gte": oneDayAgo,
		},
	}
	tradesSort := bson.M{
		"price": t,
	}

	trades, err := svc.tradeRepository.Find(tradesQuery, tradesSort, 0, -1)
	if err != nil {
		panic(err)
	}

	return trades
}

func (svc wsOrderbookService) _get24HoursTrades(o types.GetOrderBook) []*_engineTypes.Trade {
	currentTime := time.Now()
	oneDayAgo := currentTime.AddDate(0, 0, -1)

	tradesQuery := bson.M{
		"underlying":  o.Underlying,
		"strikePrice": o.StrikePrice,
		"expiryDate":  o.ExpiryDate,
		"createdAt": bson.M{
			"$gte": oneDayAgo,
		},
	}
	tradesSort := bson.M{
		"createdAt": 1,
	}

	trades, err := svc.tradeRepository.Find(tradesQuery, tradesSort, 0, -1)
	if err != nil {
		panic(err)
	}

	return trades
}
