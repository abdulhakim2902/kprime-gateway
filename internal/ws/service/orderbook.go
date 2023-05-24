package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	_deribitModel "gateway/internal/deribit/model"
	_engineTypes "gateway/internal/engine/types"
	_orderbookTypes "gateway/internal/orderbook/types"
	_tradeType "gateway/internal/repositories/types"

	"gateway/internal/repositories"
	"gateway/pkg/redis"
	"gateway/pkg/utils"
	"gateway/pkg/ws"

	"github.com/Shopify/sarama"
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

func (svc wsOrderbookService) SubscribeBook(c *ws.Client, channel string) {
	s := strings.Split(channel, ".")
	instrument := s[1]
	socket := ws.GetBookSocket()
	_string := instrument
	substring := strings.Split(_string, "-")

	_strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		fmt.Println(err)
		return
	}
	_underlying := substring[0]
	_expiryDate := strings.ToUpper(substring[1])

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: _string,
		Underlying:     _underlying,
		ExpiryDate:     _expiryDate,
		StrikePrice:    _strikePrice,
	}

	// Subscribe
	id := fmt.Sprintf("%s-%s", instrument, s[2])
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
	res, err := svc.redis.GetValue("CHANGEID-" + _string)
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
	switch s[2] {
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
		if s[2] == "agg2" {
			changeIdData = _orderbookTypes.Change{
				Id:            1,
				Timestamp:     ts,
				TimestampPrev: ts,
				AsksAgg:       changeAsksRaw,
				BidsAgg:       changeBidsRaw,
			}
		} else if s[2] == "100ms" {
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
		svc.redis.Set("CHANGEID-"+_string, string(jsonBytes))
		changeId = _orderbookTypes.Change{
			Id:            1,
			Timestamp:     ts,
			TimestampPrev: ts,
		}
	} else {
		if s[2] == "agg2" {
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

				svc.redis.Set("CHANGEID-"+_string, string(jsonBytes))
			}
		} else if s[2] == "100ms" {
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

				svc.redis.Set("CHANGEID-"+_string, string(jsonBytes))
			}
		}
	}

	if s[2] == "100ms" {
		svc.redis.Set("SNAPSHOTID-"+_string, strconv.Itoa(changeId.Id))
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

func (svc wsOrderbookService) Unsubscribe(c *ws.Client) {
	socket := ws.GetOrderBookSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderbookService) UnsubscribeQuote(c *ws.Client) {
	socket := ws.GetQuoteSocket()
	socket.Unsubscribe(c)
}

func (svc wsOrderbookService) SubscribeUserChange(c *ws.Client, channel string, userId string) {
	socket := ws.GetOrderBookSocket()
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
	return
}

func (svc wsOrderbookService) HandleConsumeUserChange(msg *sarama.ConsumerMessage) {
	var data _engineTypes.EngineResponse
	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
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
	ordersInterface := make([]interface{}, 0)
	for _, order := range orders {
		ordersInterface = append(ordersInterface, order)
	}
	response := _orderbookTypes.ChangeResponse{
		InstrumentName: _instrument,
		Trades:         tradesInterface,
		Orders:         ordersInterface,
	}

	keys = make(map[interface{}]bool)
	for _, id := range userId {
		if _, ok := keys[id]; !ok {
			keys[id] = true
			mapIndex := fmt.Sprintf("%s-%s", _instrument, id)
			if _, ok := userChanges[mapIndex]; !ok {
				userChangesMutex.Lock()
				userChanges[mapIndex] = ordersInterface
				userChangesTrades[mapIndex] = tradesInterface
				userChangesMutex.Unlock()
				go svc.HandleConsumeUserChange100ms(_instrument, id.(string))
			} else {
				userChangesMutex.Lock()
				userChanges[mapIndex] = append(userChanges[mapIndex], ordersInterface...)
				userChangesTrades[mapIndex] = append(userChangesTrades[mapIndex], tradesInterface...)
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
	return
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

func (svc wsOrderbookService) GetOrderBook(ctx context.Context, data _deribitModel.DeribitGetOrderBookRequest) _deribitModel.DeribitGetOrderBookResponse {
	instruments, _ := utils.ParseInstruments(data.InstrumentName)

	_order := _orderbookTypes.GetOrderBook{
		InstrumentName: data.InstrumentName,
		Underlying:     instruments.Underlying,
		ExpiryDate:     instruments.ExpDate,
		StrikePrice:    instruments.Strike,
	}

	dataQuote, orderBook := svc.GetDataQuote(_order)

	//check state
	dateString := instruments.ExpDate
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
			_volumeAmount += item.Amount
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

	_getImplieds := svc.tradeRepository.GetImpliedVolatility()

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
		Bids_iv:        _getImplieds.Bid,
		Asks_iv:        _getImplieds.Ask,
		Stats: _deribitModel.OrderBookStats{
			High:        _hightPrice,
			Low:         _lowestPrice,
			PriceChange: _priceChange,
			Volume:      _volumeAmount,
		},
	}

	_getIndexPrice := svc.rawPriceRepository.GetLatestIndexPrice(_order)
	if len(_getIndexPrice) > 0 {
		results.IndexPrice = &_getIndexPrice[0].Price
		results.UnderlyingIndex = &_getIndexPrice[0].Price
	}

	_getSettlementPrice := svc.settlementPriceRepository.GetLatestSettlementPrice(_order)
	if len(_getSettlementPrice) > 0 {
		results.SettlementPrice = &_getSettlementPrice[0].Price
	}

	return results
}

func (svc wsOrderbookService) GetOrderLatestTimestamp(o _orderbookTypes.GetOrderBook, after int64, isFilled bool) _orderbookTypes.Orderbook {
	return svc.orderRepository.GetOrderLatestTimestamp(o, after, isFilled)
}

func (svc wsOrderbookService) GetOrderLatestTimestampAgg(o _orderbookTypes.GetOrderBook, after int64) _orderbookTypes.Orderbook {
	return svc.orderRepository.GetOrderLatestTimestampAgg(o, after)
}
