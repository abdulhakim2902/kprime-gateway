package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	_deribitModel "gateway/internal/deribit/model"
	_engineTypes "gateway/internal/engine/types"
	_orderbookTypes "gateway/internal/orderbook/types"
	_tradeType "gateway/internal/repositories/types"

	orderType "github.com/Undercurrent-Technologies/kprime-utilities/models/order"

	"gateway/internal/repositories"
	"gateway/pkg/memdb"
	"gateway/pkg/redis"
	"gateway/pkg/utils"
	"gateway/pkg/ws"

	"github.com/Shopify/sarama"
	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

var userChangesMutex sync.RWMutex
var userChanges = make(map[string][]interface{})
var userChangesTrades = make(map[string][]interface{})

var tickerMutex sync.RWMutex
var tickerChanges = make(map[string][]interface{})

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
	ins, err := utils.ParseInstruments(instrument, false)
	if err != nil {
		msg := map[string]string{"Message": fmt.Sprintf("invalid instrument '%s'", instrument)}
		socket.SendErrorMessage(c, msg)
		return
	}

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: instrument,
		Underlying:     ins.Underlying,
		ExpiryDate:     ins.ExpDate,
		StrikePrice:    ins.Strike,
	}

	// Get initial data from the redis
	var initData _orderbookTypes.QuoteMessage
	res, err := svc.redis.GetValue("QUOTE-" + instrument)
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

func (svc wsOrderbookService) SubscribeBook(c *ws.Client, channel, instrument, interval string) {
	socket := ws.GetBookSocket()
	ins, err := utils.ParseInstruments(instrument, false)
	if err != nil {
		msg := map[string]string{"Message": fmt.Sprintf("invalid instrument '%s'", instrument)}
		socket.SendErrorMessage(c, msg)
		return
	}

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: instrument,
		Underlying:     ins.Underlying,
		ExpiryDate:     ins.ExpDate,
		StrikePrice:    ins.Strike,
	}

	// Subscribe
	id := fmt.Sprintf("%s-%s", instrument, interval)
	err = socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))

	ts := time.Now().UnixNano() / int64(time.Millisecond)
	var changeId _orderbookTypes.Change
	// Get change_id
	res, err := svc.redis.GetValue("CHANGEID-" + instrument)
	if res == "" || err != nil {
		changeId.Timestamp = ts
	} else {
		err = json.Unmarshal([]byte(res), &changeId)
		if err != nil {
			msg := map[string]string{"Message": err.Error()}
			socket.SendErrorMessage(c, msg)
			return
		}
	}

	// Get initial data from db
	var orderBook _orderbookTypes.Orderbook
	switch interval {
	case "raw":
		orderBook = svc.GetOrderLatestTimestamp(_order, changeId.Timestamp, false)
	case "100ms":
		orderBook = svc.GetOrderLatestTimestamp(_order, changeId.Timestamp, false)
	case "agg2":
		orderBook = svc.orderRepository.GetOrderBookAgg2(_order)
	}

	var bidsData [][]interface{}
	var asksData [][]interface{}

	var changeAsksRaw = make(map[string]float64)
	var changeBidsRaw = make(map[string]float64)

	if len(orderBook.Asks) > 0 {
		for _, ask := range orderBook.Asks {
			var askData []interface{}
			askData = append(askData, "new")
			askData = append(askData, ask.Price)
			askData = append(askData, ask.Amount)
			asksData = append(asksData, askData)
			changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
		}
	} else {
		asksData = make([][]interface{}, 0)
	}
	if len(orderBook.Bids) > 0 {
		for _, bid := range orderBook.Bids {
			var bidData []interface{}
			bidData = append(bidData, "new")
			bidData = append(bidData, bid.Price)
			bidData = append(bidData, bid.Amount)
			bidsData = append(bidsData, bidData)
			changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
		}
	} else {
		bidsData = make([][]interface{}, 0)
	}

	if res == "" {
		// Set initial data if null
		var changeIdData _orderbookTypes.Change
		if interval == "agg2" {
			changeIdData = _orderbookTypes.Change{
				Id:            1,
				Timestamp:     ts,
				TimestampPrev: ts,
				AsksAgg:       changeAsksRaw,
				BidsAgg:       changeBidsRaw,
			}
		} else if interval == "100ms" {
			changeIdData = _orderbookTypes.Change{
				Id:            1,
				Timestamp:     ts,
				TimestampPrev: ts,
				Asks100:       changeAsksRaw,
				Bids100:       changeBidsRaw,
			}
		} else {
			changeIdData = _orderbookTypes.Change{
				Id:            1,
				Timestamp:     ts,
				TimestampPrev: ts,
				Asks:          changeAsksRaw,
				Bids:          changeBidsRaw,
			}
		}

		//convert changeIdData to json
		jsonBytes, err := json.Marshal(changeIdData)
		if err != nil {
			fmt.Println(err)
			return
		}
		svc.redis.Set("CHANGEID-"+instrument, string(jsonBytes))
		changeId = _orderbookTypes.Change{
			Id:            1,
			Timestamp:     ts,
			TimestampPrev: ts,
		}
	} else {
		if interval == "agg2" {
			if len(changeId.AsksAgg) == 0 && len(changeId.BidsAgg) == 0 {
				changeIdData := _orderbookTypes.Change{
					Id:            changeId.Id,
					IdPrev:        changeId.IdPrev,
					Timestamp:     changeId.Timestamp,
					TimestampPrev: changeId.TimestampPrev,
					Bids:          changeId.Bids,
					Asks:          changeId.Asks,
					Asks100:       changeId.Asks100,
					Bids100:       changeId.Bids100,
					AsksAgg:       changeAsksRaw,
					BidsAgg:       changeBidsRaw,
				}
				//convert changeIdData to json
				jsonBytes, err := json.Marshal(changeIdData)
				if err != nil {
					fmt.Println(err)
					return
				}

				svc.redis.Set("CHANGEID-"+instrument, string(jsonBytes))
			}
		} else if interval == "100ms" {
			if len(changeId.Asks100) == 0 && len(changeId.Bids100) == 0 {
				changeIdData := _orderbookTypes.Change{
					Id:            changeId.Id,
					IdPrev:        changeId.IdPrev,
					Timestamp:     changeId.Timestamp,
					TimestampPrev: changeId.TimestampPrev,
					Bids:          changeId.Bids,
					Asks:          changeId.Asks,
					Asks100:       changeAsksRaw,
					Bids100:       changeBidsRaw,
					AsksAgg:       changeId.AsksAgg,
					BidsAgg:       changeId.BidsAgg,
				}
				//convert changeIdData to json
				jsonBytes, err := json.Marshal(changeIdData)
				if err != nil {
					fmt.Println(err)
					return
				}

				svc.redis.Set("CHANGEID-"+instrument, string(jsonBytes))
			}
		}
	}

	if interval == "100ms" {
		svc.redis.Set("SNAPSHOTID-"+instrument, strconv.Itoa(changeId.Id))
	}

	bookData := _orderbookTypes.BookData{
		Type:           "snapshot",
		Timestamp:      changeId.Timestamp,
		InstrumentName: instrument,
		ChangeId:       changeId.Id,
		Bids:           bidsData,
		Asks:           asksData,
	}

	params := _orderbookTypes.QuoteResponse{
		Channel: channel,
		Data:    bookData,
	}
	method := "subscription"
	// Send initial data
	socket.SendInitMessage(c, method, params)
}

func (svc wsOrderbookService) SubscribeTicker(c *ws.Client, channel, instrument, interval string) {
	socket := ws.GetBookSocket()
	ins, err := utils.ParseInstruments(instrument, false)
	if err != nil {
		msg := map[string]string{"Message": fmt.Sprintf("invalid instrument '%s'", instrument)}
		socket.SendErrorMessage(c, msg)
		return
	}

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: instrument,
		Underlying:     ins.Underlying,
		ExpiryDate:     ins.ExpDate,
		StrikePrice:    ins.Strike,
	}

	// Subscribe
	id := fmt.Sprintf("ticker-%s-%s", instrument, interval)
	err = socket.Subscribe(id, c)
	if err != nil {
		msg := map[string]string{"Message": err.Error()}
		socket.SendErrorMessage(c, msg)
		return
	}

	// Prepare when user is doing unsubscribe
	ws.RegisterConnectionUnsubscribeHandler(c, socket.UnsubscribeHandler(id))

	ts := time.Now().UnixNano() / int64(time.Millisecond)
	var changeId _orderbookTypes.Change
	// Get change_id
	res, err := svc.redis.GetValue("CHANGEID-" + instrument)
	if res == "" || err != nil {
		changeId.Timestamp = ts
	} else {
		err = json.Unmarshal([]byte(res), &changeId)
		if err != nil {
			msg := map[string]string{"Message": err.Error()}
			socket.SendErrorMessage(c, msg)
			return
		}
	}

	// Get initial data from db
	var orderBook _orderbookTypes.Orderbook
	switch interval {
	case "raw":
		orderBook = svc.GetOrderLatestTimestamp(_order, changeId.Timestamp, false)
	case "100ms":
		orderBook = svc.GetOrderLatestTimestamp(_order, changeId.Timestamp, false)
	case "agg2":
		orderBook = svc.orderRepository.GetOrderBookAgg2(_order)
	}

	dataQuote := svc.GetBestPrice(orderBook, instrument)

	orderBookValue, indexPrice, markData := svc.GetDataOrderBook(_order, dataQuote)

	results := _deribitModel.TickerSubcriptionResponse{
		InstrumentName: orderBook.InstrumentName,
		BestAskPrice:   dataQuote.BestAskPrice,
		BestAskAmount:  dataQuote.BestAskAmount,
		BestBidPrice:   dataQuote.BestBidPrice,
		BestBidAmount:  dataQuote.BestBidAmount,
		Timestamp:      time.Now().UnixNano() / int64(time.Millisecond),
		State:          orderBookValue.State,
		LastPrice:      orderBookValue.LastPrice,
		Bids_iv:        orderBookValue.ImpliedBid,
		Asks_iv:        orderBookValue.ImpliedAsk,
		Stats: _deribitModel.OrderBookStats{
			High:        orderBookValue.HighestPrice,
			Low:         orderBookValue.LowestPrice,
			PriceChange: orderBookValue.PriceChange,
			Volume:      orderBookValue.VolumeAmount,
		},
		Greeks: _deribitModel.OrderBookGreek{
			Delta: orderBookValue.GreeksDelta,
			Vega:  orderBookValue.GreeksVega,
			Gamma: orderBookValue.GreeksGamma,
			Tetha: orderBookValue.GreeksTetha,
			Rho:   orderBookValue.GreeksRho,
		},
		MarkPrice: &markData.MarkPrice,
		MarkIv:    &markData.MarkIv,
	}

	if markData.MarkPrice == 0 {
		results.MarkPrice = nil
		results.MarkIv = nil
	}
	if len(indexPrice) > 0 {
		results.IndexPrice = &indexPrice[0].Price
		results.UnderlyingPrice = &indexPrice[0].Price
	}
	results.UnderlyingIndex = "index_price"

	_getSettlementPrice := svc.settlementPriceRepository.GetLatestSettlementPrice(
		_order.Underlying,
		_order.ExpiryDate,
	)
	if len(_getSettlementPrice) > 0 {
		results.SettlementPrice = &_getSettlementPrice[0].Price
	}

	params := _orderbookTypes.QuoteResponse{
		Channel: channel,
		Data:    results,
	}
	method := "subscription"
	// Send initial data
	socket.SendInitMessage(c, method, params)
}

func (svc wsOrderbookService) GetDataOrderBook(_order _orderbookTypes.GetOrderBook, dataQuote _orderbookTypes.QuoteMessage) (_orderbookTypes.OrderBookData, []*_engineTypes.RawPrice, _orderbookTypes.MarkData) {
	//check state
	dateString := _order.ExpiryDate
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

	//TODO query to get expires time
	expiredDate := dateString
	currentDate := time.Now().Format("2006-01-02 15:04:05")
	layoutExpired := "02Jan06"
	layoutCurrent := "2006-01-02 15:04:05"

	expired, _ := time.Parse(layoutExpired, expiredDate)
	current, _ := time.Parse(layoutCurrent, currentDate)
	calculate := float64(expired.Day()) - float64(current.Day())

	dateValue := float64(calculate / 365)

	//TODO query to trades collections
	_getLastTrades := svc.tradeRepository.GetLastTrades(_order)
	_lastPrice := 0.0
	if len(_getLastTrades) > 0 {
		_lastPrice = _getLastTrades[len(_getLastTrades)-1].Price
	}

	_getHigestTrade := svc.tradeRepository.GetHighLowTrades(_order, -1)
	_hightPrice := 0.0
	if len(_getHigestTrade) > 0 {
		_hightPrice = _getHigestTrade[0].Price
	}

	_getLowestTrade := svc.tradeRepository.GetHighLowTrades(_order, 1)
	_lowestPrice := 0.0
	_volumeAmount := 0.0
	if len(_getLowestTrade) > 0 {
		_lowestPrice = _getLowestTrade[0].Price
		for _, item := range _getLowestTrade {
			// Convert string to float
			conversion, _ := utils.ConvertToFloat(item.Amount)
			_volumeAmount += conversion
		}
	}

	_get24HoursTrade := svc.tradeRepository.Get24HoursTrades(_order)
	_priceChange := 0.0
	if len(_get24HoursTrade) > 0 {
		_firstTrade := _get24HoursTrade[0].Price
		_lastTrade := _get24HoursTrade[len(_get24HoursTrade)-1].Price

		//calculate price change with percentage
		_priceChange = (_lastTrade - _firstTrade) / _firstTrade * 100
	}

	//TODO query to get Underlying Price
	var underlyingPrice float64
	_getIndexPrice := svc.rawPriceRepository.GetLatestIndexPrice(_order)
	if len(_getIndexPrice) > 0 {
		underlyingPrice = float64(_getIndexPrice[0].Price)
	} else {
		underlyingPrice = float64(0)
	}

	//TODO query to get Option Price
	str := _order.InstrumentName
	parts := strings.Split(str, "-")
	lastPart := parts[len(parts)-1]
	optionPrice := ""
	if string(lastPart[0]) == "C" {
		optionPrice = "call"
	} else {
		optionPrice = "put"
	}

	//TODO query to get ask_iv and bid_iv
	_getImpliedsAsk := svc.tradeRepository.GetImpliedVolatility(float64(dataQuote.BestAskAmount), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getImpliedsBid := svc.tradeRepository.GetImpliedVolatility(float64(dataQuote.BestBidAmount), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))

	//TODO query to get all greeks
	_getImpliedsVolatility := svc.tradeRepository.GetImpliedVolatility(float64(_lastPrice), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksDelta := svc.tradeRepository.GetGreeks("delta", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksVega := svc.tradeRepository.GetGreeks("vega", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksGamma := svc.tradeRepository.GetGreeks("gamma", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksTetha := svc.tradeRepository.GetGreeks("tetha", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksRho := svc.tradeRepository.GetGreeks("rho", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))

	value := _orderbookTypes.OrderBookData{
		State:        _state,
		HighestPrice: _hightPrice,
		LastPrice:    _lastPrice,
		LowestPrice:  _lowestPrice,
		PriceChange:  _priceChange,
		ImpliedAsk:   _getImpliedsAsk,
		ImpliedBid:   _getImpliedsBid,
		VolumeAmount: _volumeAmount,
		GreeksDelta:  _getGreeksDelta,
		GreeksVega:   _getGreeksVega,
		GreeksGamma:  _getGreeksGamma,
		GreeksTetha:  _getGreeksTetha,
		GreeksRho:    _getGreeksRho,
	}

	markData := _orderbookTypes.MarkData{}
	if dataQuote.BestAskPrice != 0 && dataQuote.BestBidPrice != 0 {
		markData.MarkPrice = (dataQuote.BestAskPrice + dataQuote.BestBidPrice) / 2
		markData.MarkIv = svc.tradeRepository.GetImpliedVolatility(float64(markData.MarkPrice), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	}

	return value, _getIndexPrice, markData
}

func (svc wsOrderbookService) GetDataQuote(order _orderbookTypes.GetOrderBook) (_orderbookTypes.QuoteMessage, _orderbookTypes.Orderbook) {

	// Get initial data
	_getOrderBook := svc.orderRepository.GetOrderBook(order)

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

func (svc wsOrderbookService) GetBestPrice(_getOrderBook _orderbookTypes.Orderbook, instrumentName string) _orderbookTypes.QuoteMessage {
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
		Instrument:    instrumentName,
		BestAskAmount: maxAskAmount,
		BestAskPrice:  maxAskPrice,
		BestBidAmount: maxBidAmount,
		BestBidPrice:  maxBidPrice,
		Timestamp:     time.Now().UnixNano() / int64(time.Millisecond),
	}

	return initData
}

func (svc wsOrderbookService) Unsubscribe(c *ws.Client) {
	socket := ws.GetOrderBookSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderbookService) UnsubscribeQuote(c *ws.Client) {
	socket := ws.GetQuoteSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderbookService) UnsubscribeBook(c *ws.Client) {
	socket := ws.GetBookSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderbookService) SubscribeUserChange(c *ws.Client, channel string, userId string) {
	socket := ws.GetOrderBookSocket()
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
	return
}

func (svc wsOrderbookService) HandleConsumeUserChange(msg *sarama.ConsumerMessage) {
	var data _engineTypes.EngineResponse
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

	var trades _tradeType.UserTradesByInstrumentResult
	if len(data.Matches.Trades) > 0 {
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
		trades, err = svc.tradeRepository.FindUserTradesById(
			_instrument,
			userId,
			tradeId,
		)
		if err != nil {
			return
		}
	}

	orders, err := svc.orderRepository.GetChangeOrdersByInstrument(
		_instrument,
		userId,
		orderId,
	)
	if err != nil {
		return
	}
	tradesInterface := make([]interface{}, 0)
	for _, trade := range trades.Trades {
		tradesInterface = append(tradesInterface, trade)
	}

	keys = make(map[interface{}]bool)
	for _, id := range userId {
		_id := id.(primitive.ObjectID).Hex()
		if _, ok := keys[_id]; !ok {
			keys[_id] = true

			ordersInterface := make([]interface{}, 0)
			for _, order := range orders {
				if _id == order.UserId.Hex() {
					ordersInterface = append(ordersInterface, order)
				}
			}
			response := _orderbookTypes.ChangeResponse{
				InstrumentName: _instrument,
				Trades:         tradesInterface,
				Orders:         ordersInterface,
			}

			mapIndex := fmt.Sprintf("%s-%s", _instrument, _id)
			if _, ok := userChanges[mapIndex]; !ok {
				userChangesMutex.Lock()
				userChanges[mapIndex] = ordersInterface
				userChangesTrades[mapIndex] = tradesInterface
				userChangesMutex.Unlock()
				go svc.HandleConsumeUserChange100ms(_instrument, _id)
			} else {
				userChangesMutex.Lock()
				userChanges[mapIndex] = append(userChanges[mapIndex], ordersInterface...)
				userChangesTrades[mapIndex] = append(userChangesTrades[mapIndex], tradesInterface...)
				userChangesMutex.Unlock()
			}
			// broadcast to user id
			broadcastId := fmt.Sprintf("%s.%s.%s-%s", "user", "changes", _instrument, _id)

			params := _orderbookTypes.QuoteResponse{
				Channel: fmt.Sprintf("user.changes.%s.raw", _instrument),
				Data:    response,
			}
			method := "subscription"
			ws.GetOrderBookSocket().BroadcastMessageSubcription(broadcastId, method, params)
		}
	}
}

func (svc wsOrderbookService) HandleConsumeUserChangeCancel(order orderType.Order) {
	_instrument := order.Underlying + "-" + order.ExpiryDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0])

	var orderId []interface{}
	var userId []interface{}
	orderId = append(orderId, order.ID)
	userId = append(userId, order.UserID)

	orders, err := svc.orderRepository.GetChangeOrdersByInstrument(
		_instrument,
		userId,
		orderId,
	)
	if err != nil {
		return
	}
	tradesInterface := make([]interface{}, 0)

	keys := make(map[interface{}]bool)
	for _, _id := range userId {
		id := _id.(primitive.ObjectID).Hex()
		if _, ok := keys[id]; !ok {
			keys[id] = true

			ordersInterface := make([]interface{}, 0)
			for _, order := range orders {
				if _id == order.UserId.Hex() {
					ordersInterface = append(ordersInterface, order)
				}
			}
			response := _orderbookTypes.ChangeResponse{
				InstrumentName: _instrument,
				Trades:         tradesInterface,
				Orders:         ordersInterface,
			}

			mapIndex := fmt.Sprintf("%s-%s", _instrument, id)
			if _, ok := userChanges[mapIndex]; !ok {
				userChangesMutex.Lock()
				userChanges[mapIndex] = ordersInterface
				userChangesMutex.Unlock()
				go svc.HandleConsumeUserChange100ms(_instrument, id)
			} else {
				userChangesMutex.Lock()
				userChanges[mapIndex] = append(userChanges[mapIndex], ordersInterface...)
				userChangesMutex.Unlock()
			}
			// broadcast to user id
			broadcastId := fmt.Sprintf("%s.%s.%s-%s", "user", "changes", _instrument, id)

			params := _orderbookTypes.QuoteResponse{
				Channel: fmt.Sprintf("user.changes.%s.raw", _instrument),
				Data:    response,
			}
			method := "subscription"
			ws.GetOrderBookSocket().BroadcastMessageSubcription(broadcastId, method, params)
		}
	}
}

func (svc wsOrderbookService) HandleConsumeUserChange100ms(instrument string, userId string) {
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
				userChangesMutex.RLock()
				changes := userChanges[mapIndex]
				userChangesMutex.RUnlock()
				if len(changes) > 0 {
					trades := userChangesTrades[mapIndex]
					response := _orderbookTypes.ChangeResponse{
						InstrumentName: instrument,
						Trades:         trades,
						Orders:         changes,
					}
					broadcastId := fmt.Sprintf("%s.%s.%s-%s-100ms", "user", "changes", instrument, userId)
					params := _orderbookTypes.QuoteResponse{
						Channel: fmt.Sprintf("user.changes.%s.100ms", instrument),
						Data:    response,
					}
					method := "subscription"
					ws.GetOrderBookSocket().BroadcastMessageSubcription(broadcastId, method, params)
					userChangesMutex.Lock()
					userChanges[mapIndex] = make([]interface{}, 0)
					userChangesTrades[mapIndex] = make([]interface{}, 0)
					userChangesMutex.Unlock()
				}
			}
		}
	}()
}

func (svc wsOrderbookService) HandleConsumeTicker(_instrument string, interval string) {
	instruments, _ := utils.ParseInstruments(_instrument, false)

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: _instrument,
		Underlying:     instruments.Underlying,
		ExpiryDate:     instruments.ExpDate,
		StrikePrice:    instruments.Strike,
	}

	ts := time.Now().UnixNano() / int64(time.Millisecond)
	// Get latest data from db
	var orderBook _orderbookTypes.Orderbook
	if interval == "raw" {
		orderBook = svc.GetOrderLatestTimestamp(_order, ts, false)
	} else if interval == "agg2" {
		orderBook = svc.GetOrderLatestTimestampAgg(_order, ts)
	}

	dataQuote := svc.GetBestPrice(orderBook, _instrument)

	//check state
	orderBookValue, indexPrice, markData := svc.GetDataOrderBook(_order, dataQuote)

	results := _deribitModel.TickerSubcriptionResponse{
		InstrumentName: orderBook.InstrumentName,
		BestAskPrice:   dataQuote.BestAskPrice,
		BestAskAmount:  dataQuote.BestAskAmount,
		BestBidPrice:   dataQuote.BestBidPrice,
		BestBidAmount:  dataQuote.BestBidAmount,
		Timestamp:      time.Now().UnixNano() / int64(time.Millisecond),
		State:          orderBookValue.State,
		LastPrice:      orderBookValue.LastPrice,
		Bids_iv:        orderBookValue.ImpliedBid,
		Asks_iv:        orderBookValue.ImpliedAsk,
		Stats: _deribitModel.OrderBookStats{
			High:        orderBookValue.HighestPrice,
			Low:         orderBookValue.LowestPrice,
			PriceChange: orderBookValue.PriceChange,
			Volume:      orderBookValue.VolumeAmount,
		},
		Greeks: _deribitModel.OrderBookGreek{
			Delta: orderBookValue.GreeksDelta,
			Vega:  orderBookValue.GreeksVega,
			Gamma: orderBookValue.GreeksGamma,
			Tetha: orderBookValue.GreeksTetha,
			Rho:   orderBookValue.GreeksRho,
		},
		MarkPrice: &markData.MarkPrice,
		MarkIv:    &markData.MarkIv,
	}

	if markData.MarkPrice == 0 {
		results.MarkPrice = nil
		results.MarkIv = nil
	}

	if len(indexPrice) > 0 {
		results.IndexPrice = &indexPrice[0].Price
		results.UnderlyingPrice = &indexPrice[0].Price
	}
	results.UnderlyingIndex = "index_price"

	_getSettlementPrice := svc.settlementPriceRepository.GetLatestSettlementPrice(
		_order.Underlying,
		_order.ExpiryDate,
	)
	if len(_getSettlementPrice) > 0 {
		results.SettlementPrice = &_getSettlementPrice[0].Price
	}

	if interval == "raw" {
		go svc.HandleConsumeTicker(_instrument, "agg2")

		instrumentInterface := make([]interface{}, 0)
		instrumentInterface = append(instrumentInterface, _instrument)
		if _, ok := tickerChanges[_instrument]; !ok {
			tickerMutex.Lock()
			tickerChanges[_instrument] = instrumentInterface
			tickerMutex.Unlock()
			go svc.HandleConsumeUserTicker100ms(_instrument)
		} else {
			tickerMutex.Lock()
			tickerChanges[_instrument] = instrumentInterface
			tickerMutex.Unlock()
		}
	}
	params := _orderbookTypes.QuoteResponse{
		Channel: fmt.Sprintf("ticker.%s.%s", _instrument, interval),
		Data:    results,
	}
	method := "subscription"
	broadcastId := fmt.Sprintf("ticker-%s-%s", _instrument, interval)
	ws.GetBookSocket().BroadcastMessage(broadcastId, method, params)
}

func (svc wsOrderbookService) HandleConsumeUserTicker100ms(instrument string) {
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
				tickerMutex.RLock()
				changes := tickerChanges[instrument]
				tickerMutex.RUnlock()
				if len(changes) > 0 {
					tickerMutex.Lock()
					tickerChanges[instrument] = make([]interface{}, 0)
					tickerMutex.Unlock()

					instruments, _ := utils.ParseInstruments(instrument, false)

					_order := _orderbookTypes.GetOrderBook{
						InstrumentName: instrument,
						Underlying:     instruments.Underlying,
						ExpiryDate:     instruments.ExpDate,
						StrikePrice:    instruments.Strike,
					}

					// Get latest data from db
					ts := time.Now().UnixNano() / int64(time.Millisecond)
					orderBook := svc.GetOrderLatestTimestamp(_order, ts, false)

					dataQuote := svc.GetBestPrice(orderBook, instrument)

					orderBookValue, indexPrice, markData := svc.GetDataOrderBook(_order, dataQuote)

					results := _deribitModel.TickerSubcriptionResponse{
						InstrumentName: orderBook.InstrumentName,
						BestAskPrice:   dataQuote.BestAskPrice,
						BestAskAmount:  dataQuote.BestAskAmount,
						BestBidPrice:   dataQuote.BestBidPrice,
						BestBidAmount:  dataQuote.BestBidAmount,
						Timestamp:      time.Now().UnixNano() / int64(time.Millisecond),
						State:          orderBookValue.State,
						LastPrice:      orderBookValue.LastPrice,
						Bids_iv:        orderBookValue.ImpliedBid,
						Asks_iv:        orderBookValue.ImpliedAsk,
						Stats: _deribitModel.OrderBookStats{
							High:        orderBookValue.HighestPrice,
							Low:         orderBookValue.LowestPrice,
							PriceChange: orderBookValue.PriceChange,
							Volume:      orderBookValue.VolumeAmount,
						},
						Greeks: _deribitModel.OrderBookGreek{
							Delta: orderBookValue.GreeksDelta,
							Vega:  orderBookValue.GreeksVega,
							Gamma: orderBookValue.GreeksGamma,
							Tetha: orderBookValue.GreeksTetha,
							Rho:   orderBookValue.GreeksRho,
						},
						MarkPrice: &markData.MarkPrice,
						MarkIv:    &markData.MarkIv,
					}

					if markData.MarkPrice == 0 {
						results.MarkPrice = nil
						results.MarkIv = nil
					}

					if len(indexPrice) > 0 {
						results.IndexPrice = &indexPrice[0].Price
						results.UnderlyingPrice = &indexPrice[0].Price
					}
					results.UnderlyingIndex = "index_price"

					_getSettlementPrice := svc.settlementPriceRepository.GetLatestSettlementPrice(
						_order.Underlying,
						_order.ExpiryDate,
					)
					if len(_getSettlementPrice) > 0 {
						results.SettlementPrice = &_getSettlementPrice[0].Price
					}

					broadcastId := fmt.Sprintf("ticker-%s-100ms", instrument)

					params := _orderbookTypes.QuoteResponse{
						Channel: fmt.Sprintf("ticker.%s.100ms", instrument),
						Data:    results,
					}
					method := "subscription"
					ws.GetBookSocket().BroadcastMessage(broadcastId, method, params)
				}
			}
		}
	}()
}

func (svc wsOrderbookService) GetOrderBook(ctx context.Context, data _deribitModel.DeribitGetOrderBookRequest) _deribitModel.DeribitGetOrderBookResponse {
	instruments, _ := utils.ParseInstruments(data.InstrumentName, false)

	user, _, err := memdb.MDBFindUserById(data.UserId)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")
	}

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: data.InstrumentName,
		Underlying:     instruments.Underlying,
		ExpiryDate:     instruments.ExpDate,
		StrikePrice:    instruments.Strike,
		UserId:         data.UserId,
	}

	ordExclusions := []string{}
	for _, userCast := range user.OrderExclusions {
		ordExclusions = append(ordExclusions, userCast.UserID)
	}

	_order.UserRole = user.Role.String()
	_order.UserOrderExclusions = ordExclusions

	dataQuote, orderBook := svc.GetDataQuote(_order)

	orderBookValue, indexPrice, markData := svc.GetDataOrderBook(_order, dataQuote)

	results := _deribitModel.DeribitGetOrderBookResponse{
		InstrumentName: orderBook.InstrumentName,
		Bids:           orderBook.Bids,
		Asks:           orderBook.Asks,
		BestAskPrice:   dataQuote.BestAskPrice,
		BestAskAmount:  dataQuote.BestAskAmount,
		BestBidPrice:   dataQuote.BestBidPrice,
		BestBidAmount:  dataQuote.BestBidAmount,
		Timestamp:      time.Now().UnixNano() / int64(time.Millisecond),
		State:          orderBookValue.State,
		LastPrice:      orderBookValue.LastPrice,
		Bids_iv:        orderBookValue.ImpliedBid,
		Asks_iv:        orderBookValue.ImpliedAsk,
		Stats: _deribitModel.OrderBookStats{
			High:        orderBookValue.HighestPrice,
			Low:         orderBookValue.LowestPrice,
			PriceChange: orderBookValue.PriceChange,
			Volume:      orderBookValue.VolumeAmount,
		},
		Greeks: _deribitModel.OrderBookGreek{
			Delta: orderBookValue.GreeksDelta,
			Vega:  orderBookValue.GreeksVega,
			Gamma: orderBookValue.GreeksGamma,
			Tetha: orderBookValue.GreeksTetha,
			Rho:   orderBookValue.GreeksRho,
		},
		MarkPrice: &markData.MarkPrice,
		MarkIv:    &markData.MarkIv,
	}

	if markData.MarkPrice == 0 {
		results.MarkPrice = nil
		results.MarkIv = nil
	}

	if len(indexPrice) > 0 {
		results.IndexPrice = &indexPrice[0].Price
		results.UnderlyingPrice = &indexPrice[0].Price
	}
	results.UnderlyingIndex = "index_price"

	_getSettlementPrice := svc.settlementPriceRepository.GetLatestSettlementPrice(
		_order.Underlying,
		_order.ExpiryDate,
	)
	if len(_getSettlementPrice) > 0 {
		results.SettlementPrice = &_getSettlementPrice[0].Price
	}

	return results
}

func (svc wsOrderbookService) GetLastTradesByInstrument(ctx context.Context, data _deribitModel.DeribitGetLastTradesByInstrumentRequest) _deribitModel.DeribitGetLastTradesByInstrumentResponse {
	_filteredGets := svc.tradeRepository.FilterTradesData(data)

	bsonResponse := _filteredGets

	_getLastTradesByInstrument := []_deribitModel.DeribitGetLastTradesByInstrumentValue{}

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
		tradeObjectId := jsonDoc["_id"].(primitive.ObjectID)
		conversion, _ := utils.ConvertToFloat(jsonDoc["amount"].(string))
		resultData := _deribitModel.DeribitGetLastTradesByInstrumentValue{
			Amount:         conversion,
			Direction:      jsonDoc["side"].(string),
			InstrumentName: fmt.Sprintf("%s-%s-%d-%s", underlying, expiryDate, int64(strikePrice), contracts),
			Price:          jsonDoc["price"].(float64),
			Timestamp:      time.Now().UnixNano() / int64(time.Millisecond),
			TradeId:        tradeObjectId.Hex(),
			Api:            true,
			IndexPrice:     jsonDoc["indexPrice"].(float64),
			TickDirection:  jsonDoc["tickDirection"].(int32),
			TradeSeq:       jsonDoc["tradeSequence"].(int32),
			CreatedAt:      jsonDoc["createdAt"].(primitive.DateTime).Time(),
		}

		_getLastTradesByInstrument = append(_getLastTradesByInstrument, resultData)
	}

	results := _deribitModel.DeribitGetLastTradesByInstrumentResponse{
		Trades: _getLastTradesByInstrument,
	}

	return results
}

func (svc wsOrderbookService) GetOrderLatestTimestamp(o _orderbookTypes.GetOrderBook, after int64, isFilled bool) _orderbookTypes.Orderbook {
	return svc.orderRepository.GetOrderLatestTimestamp(o, after, isFilled)
}

func (svc wsOrderbookService) GetOrderLatestTimestampAgg(o _orderbookTypes.GetOrderBook, after int64) _orderbookTypes.Orderbook {
	return svc.orderRepository.GetOrderLatestTimestampAgg(o, after)
}

func (svc wsOrderbookService) GetIndexPrice(ctx context.Context, data _deribitModel.DeribitGetIndexPriceRequest) _deribitModel.DeribitGetIndexPriceResponse {
	var indexPrice float64

	_getIndexPrice := svc.rawPriceRepository.GetIndexPrice(data.IndexName)
	if len(_getIndexPrice) > 0 {
		indexPrice = float64(_getIndexPrice[0].Price)
	} else {
		indexPrice = float64(0)
	}
	result := _deribitModel.DeribitGetIndexPriceResponse{
		IndexPrice: indexPrice,
	}
	return result
}

func (svc wsOrderbookService) GetDeliveryPrices(ctx context.Context, request _deribitModel.DeliveryPricesRequest) _deribitModel.DeliveryPricesResponse {
	_deliveryPrice, err := svc.settlementPriceRepository.GetDeliveryPrice(request)
	if err != nil {
		fmt.Println(err)
	}

	jsonBytes, err := json.Marshal(_deliveryPrice)
	if err != nil {
		fmt.Println(err)
	}

	var deliveryPrice _deribitModel.DeliveryPricesResponse
	err = json.Unmarshal([]byte(jsonBytes), &deliveryPrice)
	if err != nil {
		fmt.Println(err)
	}

	return deliveryPrice
}
