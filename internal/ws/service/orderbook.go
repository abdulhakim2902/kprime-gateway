package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	_deribitModel "gateway/internal/deribit/model"
	_engineTypes "gateway/internal/engine/types"
	_orderbookTypes "gateway/internal/orderbook/types"

	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/ws"

	"git.devucc.name/dependencies/utilities/types"
	"go.mongodb.org/mongo-driver/bson"
)

type wsOrderbookService struct {
	redis                     *redis.RedisConnectionPool
	orderRepository           *repositories.OrderRepository
	tradeRepository           *repositories.TradeRepository
	rawPriceRepository        *repositories.RawPriceRepository
	settlementPriceRepository *repositories.SettlementPriceRepository
}

func NewWSOrderbookService(redis *redis.RedisConnectionPool,
	orderRepository *repositories.OrderRepository,
	tradeRepository *repositories.TradeRepository,
	rawPriceRepository *repositories.RawPriceRepository,
	settlementPriceRepository *repositories.SettlementPriceRepository,
) IwsOrderbookService {
	return &wsOrderbookService{redis, orderRepository, tradeRepository, rawPriceRepository, settlementPriceRepository}
}

func (svc wsOrderbookService) Subscribe(c *ws.Client, instrument string) {
	socket := ws.GetOrderBookSocket()

	// Get initial data from the redis
	res, err := svc.redis.GetValue("ORDERBOOK-" + instrument)
	if res == "" || err != nil {
		socket.SendInitMessage(c, &_orderbookTypes.Message{
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
	var initData _orderbookTypes.Message
	err = json.Unmarshal([]byte(res), &initData)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Send initial data from the redis
	socket.SendInitMessage(c, initData)
}

func (svc wsOrderbookService) SubscribeQuote(c *ws.Client, instrument string) {
	socket := ws.GetQuoteSocket()
	_string := instrument
	substring := strings.Split(_string, "-")

	_strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		panic(err)
	}
	_underlying := substring[0]
	_expiryDate := strings.ToUpper(substring[1])

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: _string,
		Underlying:     _underlying,
		ExpiryDate:     _expiryDate,
		StrikePrice:    _strikePrice,
	}

	// Get initial data from the redis
	var initData _orderbookTypes.QuoteMessage
	res, err := svc.redis.GetValue("QUOTE-" + _string)
	if res == "" || err != nil {
		// Get initial data if null
		initData, _ = svc.GetDataQuote(_order)
	} else {
		err = json.Unmarshal([]byte(res), &initData)
		if err != nil {
			msg := map[string]string{"Message": err.Error()}
			socket.SendErrorMessage(c, msg)
			return
		}
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

	params := _orderbookTypes.QuoteResponse{
		Channel: fmt.Sprintf("quote.%s", instrument),
		Data:    initData,
	}
	method := "subscription"
	// Send initial data from the redis
	socket.SendInitMessage(c, method, params)
}

func (svc wsOrderbookService) GetDataQuote(order _orderbookTypes.GetOrderBook) (_orderbookTypes.QuoteMessage, _orderbookTypes.Orderbook) {

	// Get initial data
	_getOrderBook := svc._getOrderBook(order)

	//count best Ask
	maxAskPrice := 0.0
	maxAskAmount := 0.0
	var maxAskItem []*_orderbookTypes.WsOrder
	for index, item := range _getOrderBook.Asks {
		if index == 0 {
			maxAskPrice = item.Price
			maxAskItem = []*_orderbookTypes.WsOrder{item}
		}
		if item.Price < maxAskPrice {
			maxAskPrice = item.Price
			maxAskItem = []*_orderbookTypes.WsOrder{item}
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
	var maxBidItem []*_orderbookTypes.WsOrder
	for _, item := range _getOrderBook.Bids {
		if item.Price > maxBidPrice {
			maxBidPrice = item.Price
			maxBidItem = []*_orderbookTypes.WsOrder{item}
			maxBidAmount = item.Amount
		} else if item.Price == maxBidPrice {
			maxBidItem = append(maxBidItem, item)
			maxBidAmount += item.Amount
		}
	}

	initData := _orderbookTypes.QuoteMessage{
		Instrument:    order.InstrumentName,
		BestAskAmount: maxAskAmount,
		BestAskPrice:  maxAskPrice,
		BestBidAmount: maxBidAmount,
		BestBidPrice:  maxBidPrice,
		Timestamp:     time.Now().UnixNano() / int64(time.Millisecond),
	}

	bidsAsksData := _orderbookTypes.Orderbook{
		InstrumentName: order.InstrumentName,
		Bids:           _getOrderBook.Bids,
		Asks:           _getOrderBook.Asks,
	}
	// convert data to json
	jsonBytes, err := json.Marshal(initData)
	if err != nil {
		fmt.Println(err)
		return initData, bidsAsksData
	}
	svc.redis.Set("QUOTE-"+order.InstrumentName, string(jsonBytes))

	return initData, bidsAsksData
}

func (svc wsOrderbookService) Unsubscribe(c *ws.Client) {
	socket := ws.GetOrderBookSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderbookService) UnsubscribeQuote(c *ws.Client) {
	socket := ws.GetQuoteSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderbookService) GetOrderBook(ctx context.Context, data _deribitModel.DeribitGetOrderBookRequest) _deribitModel.DeribitGetOrderBookResponse {
	_string := data.InstrumentName
	substring := strings.Split(_string, "-")

	_strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		return _deribitModel.DeribitGetOrderBookResponse{}
	}
	_underlying := substring[0]
	_expiryDate := strings.ToUpper(substring[1])

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: _string,
		Underlying:     _underlying,
		ExpiryDate:     _expiryDate,
		StrikePrice:    _strikePrice,
	}

	dataQuote, orderBook := svc.GetDataQuote(_order)

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

	results := _deribitModel.DeribitGetOrderBookResponse{
		InstrumentName: orderBook.InstrumentName,
		Bids:           orderBook.Bids,
		Asks:           orderBook.Asks,
		BestAskPrice:   dataQuote.BestAskPrice,
		BestAskAmount:  dataQuote.BestAskAmount,
		BestBidPrice:   dataQuote.BestBidPrice,
		BestBidAmount:  dataQuote.BestBidAmount,
		Timestamp:      time.Now().UnixNano() / int64(time.Millisecond),
		State:          _state,
		LastPrice:      _lastPrice,
		Stats: _deribitModel.OrderBookStats{
			High:        _hightPrice,
			Low:         _lowestPrice,
			PriceChange: _priceChange,
			Volume:      _volumeAmount,
		},
	}

	_getIndexPrice := svc._getLatestIndexPrice(_order)
	if len(_getIndexPrice) > 0 {
		results.IndexPrice = &_getIndexPrice[0].Price
		results.UnderlyingIndex = &_getIndexPrice[0].Price
	}

	_getSettlementPrice := svc._getLatestSettlementPrice(_order)
	if len(_getSettlementPrice) > 0 {
		results.SettlementPrice = &_getSettlementPrice[0].Price
	}

	return results
}

func (svc wsOrderbookService) _getOrderBook(o _orderbookTypes.GetOrderBook) *_orderbookTypes.Orderbook {
	queryBuilder := func(side types.Side, priceOrder int) interface{} {
		return []bson.M{
			{
				"$match": bson.M{
					"status":      bson.M{"$in": []types.OrderStatus{types.OPEN, types.PARTIAL_FILLED}},
					"underlying":  o.Underlying,
					"strikePrice": o.StrikePrice,
					"expiryDate":  o.ExpiryDate,
					"side":        side,
				},
			},
			{
				"$group": bson.D{
					{"_id", "$price"},
					{"amount", bson.D{{"$sum", bson.M{"$subtract": []string{"$amount", "$filledAmount"}}}}},
					{"detail", bson.D{{"$first", "$$ROOT"}}},
				},
			},
			{"$sort": bson.M{"price": priceOrder, "createdAt": 1}},
			{
				"$replaceRoot": bson.D{
					{"newRoot",
						bson.D{
							{"$mergeObjects",
								bson.A{
									"$detail",
									bson.D{{"amount", "$amount"}},
								},
							},
						},
					},
				},
			},
		}
	}

	asksQuery := queryBuilder(types.SELL, -1)
	asks := svc.orderRepository.WsAggregate(asksQuery)

	bidsQuery := queryBuilder(types.BUY, 1)
	bids := svc.orderRepository.WsAggregate(bidsQuery)

	orderbooks := &_orderbookTypes.Orderbook{
		InstrumentName: o.InstrumentName,
		Asks:           asks,
		Bids:           bids,
	}

	return orderbooks
}

func (svc wsOrderbookService) _getLastTrades(o _orderbookTypes.GetOrderBook) []*_engineTypes.Trade {
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

func (svc wsOrderbookService) _getHighLowTrades(o _orderbookTypes.GetOrderBook, t int) []*_engineTypes.Trade {
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

func (svc wsOrderbookService) _get24HoursTrades(o _orderbookTypes.GetOrderBook) []*_engineTypes.Trade {
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

func (svc wsOrderbookService) _getLatestIndexPrice(o _orderbookTypes.GetOrderBook) []*_engineTypes.RawPrice {
	metadataType := "index"
	metadataPair := fmt.Sprintf("%s_usd", strings.ToLower(o.Underlying))

	tradesQuery := bson.M{
		"metadata.pair": metadataPair,
		"metadata.type": metadataType,
	}
	tradesSort := bson.M{
		"ts": -1,
	}

	trades, err := svc.rawPriceRepository.Find(tradesQuery, tradesSort, 0, 1)
	if err != nil {
		panic(err)
	}

	return trades
}

func (svc wsOrderbookService) _getLatestSettlementPrice(o _orderbookTypes.GetOrderBook) []*_engineTypes.SettlementPrice {
	metadataType := "index"
	metadataPair := fmt.Sprintf("%s_usd", strings.ToLower(o.Underlying))

	tradesQuery := bson.M{
		"metadata.pair": metadataPair,
		"metadata.type": metadataType,
	}
	tradesSort := bson.M{
		"ts": -1,
	}

	trades, err := svc.settlementPriceRepository.Find(tradesQuery, tradesSort, 0, 1)
	if err != nil {
		panic(err)
	}

	return trades
}
