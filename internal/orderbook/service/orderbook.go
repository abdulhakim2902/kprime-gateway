package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"

	orderType "github.com/Undercurrent-Technologies/kprime-utilities/models/order"

	_engineType "gateway/internal/engine/types"
	ordermatch "gateway/internal/fix-acceptor"
	"gateway/internal/orderbook/types"
	wsService "gateway/internal/ws/service"

	"gateway/pkg/redis"
	"gateway/pkg/ws"
)

type orderbookHandler struct {
	redis   *redis.RedisConnectionPool
	wsOBSvc wsService.IwsOrderbookService
}

var changeId100ms = make(map[string]types.ChangeStruct)
var changeIdMutex sync.RWMutex

func NewOrderbookHandler(r *gin.Engine, redis *redis.RedisConnectionPool, wsOBSvc wsService.IwsOrderbookService) IOrderbookService {
	return &orderbookHandler{redis, wsOBSvc}

}
func (svc orderbookHandler) HandleConsume(msg *sarama.ConsumerMessage) {
	var data types.Message

	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Convert BTC-28JAN22-50000.000000-C to BTC-28JAN22-50000-C"
	parts := strings.Split(data.Instrument, "-")
	// Parse the float value and convert it to an integer
	var floatValue float64
	fmt.Sscanf(parts[2], "%f", &floatValue)
	intValue := int(floatValue)
	instrument := fmt.Sprintf("%s-%s-%d-%s", parts[0], parts[1], intValue, parts[3])

	data.Instrument = instrument

	// Save to redis
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
		return
	}
	svc.redis.Set("ORDERBOOK-"+instrument, string(jsonBytes))

	// Broadcast
	ws.GetOrderBookSocket().BroadcastMessage(instrument, data)
}

func (svc orderbookHandler) HandleConsumeUserChange(msg *sarama.ConsumerMessage) {
	svc.wsOBSvc.HandleConsumeUserChange(msg)
}

func (svc orderbookHandler) HandleConsumeUserChangeCancel(msg *sarama.ConsumerMessage) {
	var data orderType.CancelledOrder

	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, order := range data.Data {
		svc.wsOBSvc.HandleConsumeUserChangeCancel(*order)
	}
}

func (svc orderbookHandler) HandleConsumeTicker(msg *sarama.ConsumerMessage) {
	var data _engineType.EngineResponse

	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	if data.Matches == nil && len(data.Matches.TakerOrder.Contracts) > 0 {
		return
	}

	_instrument := data.Matches.TakerOrder.Underlying + "-" + data.Matches.TakerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Matches.TakerOrder.StrikePrice) + "-" + string(data.Matches.TakerOrder.Contracts[0])

	svc.wsOBSvc.HandleConsumeTicker(_instrument, "raw")
}

func (svc orderbookHandler) HandleConsumeTickerCancel(msg *sarama.ConsumerMessage) {
	var data orderType.CancelledOrder

	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	keys := make(map[interface{}]bool)
	var instruments []string
	for _, order := range data.Data {
		_instrument := order.Underlying + "-" + order.ExpiryDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0])
		if _, ok := keys[_instrument]; !ok {
			keys[_instrument] = true
			instruments = append(instruments, _instrument)
		} else {
			continue
		}
	}
	for _, instrument := range instruments {
		svc.wsOBSvc.HandleConsumeTicker(instrument, "raw")
	}
}

func (svc orderbookHandler) HandleConsumeBook(msg *sarama.ConsumerMessage) {
	var order types.Order
	var data _engineType.EngineResponse

	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}

	if data.Matches == nil {
		return
	}

	order = *data.Matches.TakerOrder
	_instrument := data.Matches.TakerOrder.Underlying + "-" + data.Matches.TakerOrder.ExpiryDate + "-" + fmt.Sprintf("%.0f", data.Matches.TakerOrder.StrikePrice) + "-" + string(data.Matches.TakerOrder.Contracts[0])

	_order := types.GetOrderBook{
		InstrumentName: _instrument,
		Underlying:     order.Underlying,
		ExpiryDate:     order.ExpiryDate,
		StrikePrice:    order.StrikePrice,
	}

	ts := time.Now().UnixNano() / int64(time.Millisecond)
	var changeId types.Change
	var changeIdNew types.Change
	// Get saved orderbook from redis
	res, err := svc.redis.GetValue("CHANGEID-" + _instrument)
	if res == "" || err != nil {
		// Set initial data if null
		changeIdNew = types.Change{
			Id:        1,
			Timestamp: ts,
		}
	} else {
		err = json.Unmarshal([]byte(res), &changeId)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	// Get latest data from db
	orderBook := svc.wsOBSvc.GetOrderLatestTimestamp(_order, ts, false)

	var bidsData = make([][]interface{}, 0)
	var asksData = make([][]interface{}, 0)
	var changeAsksRaw = make(map[string]float64)
	var changeBidsRaw = make(map[string]float64)

	if len(orderBook.Asks) > 0 {
		for _, ask := range orderBook.Asks {
			var askData []interface{}
			// check if data from last changeId is changed or is there new data incoming
			if val, ok := changeId.Asks[fmt.Sprintf("%f", ask.Price)]; ok {
				if val != ask.Amount {
					if ask.Amount == 0 {
						askData = append(askData, "delete")
					} else {
						askData = append(askData, "change")
						changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
					}
					askData = append(askData, ask.Price)
					askData = append(askData, ask.Amount)
					asksData = append(asksData, askData)
				} else {
					changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
				}
			} else {
				if ask.Amount != 0 {
					askData = append(askData, "new")
					askData = append(askData, ask.Price)
					askData = append(askData, ask.Amount)
					asksData = append(asksData, askData)
					changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
				}
			}
		}
	} else {
		asksData = make([][]interface{}, 0)
	}

	if len(orderBook.Bids) > 0 {
		for _, bid := range orderBook.Bids {
			var bidData []interface{}
			// check if data from last changeId is changed or is there new data incoming
			if val, ok := changeId.Bids[fmt.Sprintf("%f", bid.Price)]; ok {
				if val != bid.Amount {
					if bid.Amount == 0 {
						bidData = append(bidData, "delete")
					} else {
						bidData = append(bidData, "change")
						changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
					}
					bidData = append(bidData, bid.Price)
					bidData = append(bidData, bid.Amount)
					bidsData = append(bidsData, bidData)
				} else {
					changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
				}
			} else {
				if bid.Amount != 0 {
					bidData = append(bidData, "new")
					bidData = append(bidData, bid.Price)
					bidData = append(bidData, bid.Amount)
					bidsData = append(bidsData, bidData)
					changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
				}
			}
		}
	} else {
		bidsData = make([][]interface{}, 0)
	}
	// Check on order cancel
	if order.Status == "CANCELLED" {
		// Check if price point deleted
		switch order.Side {
		case "BUY":
			if amount, ok := changeId.Bids[fmt.Sprintf("%f", order.Price)]; ok {
				if amount-order.Amount == 0 {
					var bidData []interface{}
					bidData = append(bidData, "delete")
					bidData = append(bidData, order.Price)
					bidData = append(bidData, 0)
					bidsData = append(bidsData, bidData)
				}
			}
		case "SELL":
			if amount, ok := changeId.Asks[fmt.Sprintf("%f", order.Price)]; ok {
				if amount-order.Amount == 0 {
					var askData []interface{}
					askData = append(askData, "delete")
					askData = append(askData, order.Price)
					askData = append(askData, 0)
					asksData = append(asksData, askData)
				}
			}
		}
	} else if order.Status == "FILLED" { // Check on filled order
		switch order.Side {
		case "BUY":
			if amount, ok := changeId.Asks[fmt.Sprintf("%f", order.Price)]; ok {
				if amount-order.Amount == 0 {
					var askData []interface{}
					askData = append(askData, "delete")
					askData = append(askData, order.Price)
					askData = append(askData, 0)
					asksData = append(asksData, askData)
				}
			}
		case "SELL":
			if amount, ok := changeId.Bids[fmt.Sprintf("%f", order.Price)]; ok {
				if amount-order.Amount == 0 {
					var bidData []interface{}
					bidData = append(bidData, "delete")
					bidData = append(bidData, order.Price)
					bidData = append(bidData, 0)
					bidsData = append(bidsData, bidData)
				}
			}
		}
	} else if len(order.Amendments) > 0 {
		// Check on order edit
		updated := order.Amendments[len(order.Amendments)-1].UpdatedFields
		switch order.Side {
		case "BUY":
			if val, ok := updated["price"]; ok {
				if _, ok := changeId.Bids[fmt.Sprintf("%f", val.OldValue)]; ok {
					// check if old price point deleted
					if val.OldValue != order.Price {
						var bidData []interface{}
						bidData = append(bidData, "delete")
						bidData = append(bidData, val.OldValue)
						bidData = append(bidData, 0)
						bidsData = append(bidsData, bidData)
					}
				}
			}
		case "SELL":
			if val, ok := updated["price"]; ok {
				if _, ok := changeId.Asks[fmt.Sprintf("%f", val.OldValue)]; ok {
					// check if old price point deleted
					if val.OldValue != order.Price {
						var askData []interface{}
						askData = append(askData, "delete")
						askData = append(askData, val.OldValue)
						askData = append(askData, 0)
						asksData = append(asksData, askData)
					}
				}
			}
		}
	}

	// Set new data into redis
	id := changeId.Id + 1
	changeIdNew = types.Change{
		Id:            id,
		IdPrev:        changeId.Id,
		Timestamp:     ts,
		TimestampPrev: changeId.Timestamp,
		Asks:          changeAsksRaw,
		Bids:          changeBidsRaw,
		Asks100:       changeId.Asks100,
		Bids100:       changeId.Bids100,
		AsksAgg:       changeId.AsksAgg,
		BidsAgg:       changeId.BidsAgg,
	}

	//convert changeIdNew to json
	jsonBytes, err := json.Marshal(changeIdNew)
	if err != nil {
		fmt.Println(err)
		return
	}
	svc.redis.Set("CHANGEID-"+_instrument, string(jsonBytes))

	go svc.HandleConsumeBookAgg(_instrument, order, false, nil)

	changeIdMutex.Lock()
	if _, ok := changeId100ms[_instrument]; !ok {
		changeId100ms[_instrument] = types.ChangeStruct{
			Id:         changeIdNew.Id,
			IdPrev:     changeIdNew.IdPrev,
			Status:     order.Status,
			Side:       order.Side,
			Price:      order.Price,
			Amount:     order.Amount,
			Amendments: order.Amendments,
		}
		go svc.Handle100msInterval(_instrument)
	} else {
		changeId100ms[_instrument] = types.ChangeStruct{
			Id:         changeIdNew.Id,
			IdPrev:     changeIdNew.IdPrev,
			Status:     order.Status,
			Side:       order.Side,
			Price:      order.Price,
			Amount:     order.Amount,
			Amendments: order.Amendments,
		}
	}
	changeIdMutex.Unlock()

	bookData := types.BookData{
		Type:           "change",
		Timestamp:      changeIdNew.Timestamp,
		InstrumentName: _instrument,
		ChangeId:       changeIdNew.Id,
		PrevChangeId:   changeIdNew.IdPrev,
		Bids:           bidsData,
		Asks:           asksData,
	}

	params := types.QuoteResponse{
		Channel: fmt.Sprintf("book.%s.raw", _instrument),
		Data:    bookData,
	}
	method := "subscription"
	broadcastId := fmt.Sprintf("%s-raw", _instrument)
	ws.GetBookSocket().BroadcastMessage(broadcastId, method, params)
}

func (svc orderbookHandler) HandleConsumeBookCancel(msg *sarama.ConsumerMessage) {
	var _instrument string

	var data types.CancelledOrder
	err := json.Unmarshal(msg.Value, &data)
	if err != nil {
		fmt.Println(err)
		return
	}
	orders := data.Data
	books := make(map[string]types.OrderbookMap)
	asks := make(map[float64]types.WsOrder)
	bids := make(map[float64]types.WsOrder)
	// loop to store all cancelled order to the map
	for _, order := range orders {
		_instrument = order.Underlying + "-" + order.ExpiryDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0])
		if val, ok := books[_instrument]; !ok {
			switch order.Side {
			case "BUY":
				bid := types.WsOrder{
					Amount: order.Amount,
					Price:  order.Price,
				}
				bids[order.Price] = bid
			case "SELL":
				ask := types.WsOrder{
					Amount: order.Amount,
					Price:  order.Price,
				}
				asks[order.Price] = ask
			}
			books[_instrument] = types.OrderbookMap{
				InstrumentName: _instrument,
				Bids:           bids,
				Asks:           asks,
			}
		} else {
			switch order.Side {
			case "BUY":
				bid := types.WsOrder{
					Amount: order.Amount,
					Price:  order.Price,
				}
				if v, ok := val.Bids[order.Price]; !ok {
					val.Bids[order.Price] = bid
				} else {
					v.Amount = v.Amount + order.Amount
					val.Bids[order.Price] = v
				}
			case "SELL":
				ask := types.WsOrder{
					Amount: order.Amount,
					Price:  order.Price,
				}
				if v, ok := val.Asks[order.Price]; !ok {
					val.Asks[order.Price] = ask
				} else {
					v.Amount = v.Amount + order.Amount
					val.Asks[order.Price] = v
				}
			}
			books[_instrument] = types.OrderbookMap{
				InstrumentName: _instrument,
				Bids:           bids,
				Asks:           asks,
			}
		}
	}

	keys := make(map[interface{}]bool)
	for _, order := range orders {
		_instrument = order.Underlying + "-" + order.ExpiryDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0])
		if _, ok := keys[_instrument]; !ok {
			keys[_instrument] = true
		} else {
			continue
		}

		_order := types.GetOrderBook{
			InstrumentName: _instrument,
			Underlying:     order.Underlying,
			ExpiryDate:     order.ExpiryDate,
			StrikePrice:    order.StrikePrice,
		}

		ts := time.Now().UnixNano() / int64(time.Millisecond)
		var changeId types.Change
		var changeIdNew types.Change
		// Get saved orderbook from redis
		res, err := svc.redis.GetValue("CHANGEID-" + _instrument)
		if res == "" || err != nil {
			// Set initial data if null
			changeIdNew = types.Change{
				Id:        1,
				Timestamp: ts,
			}
		} else {
			err = json.Unmarshal([]byte(res), &changeId)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
		// Get latest data from db
		orderBook := svc.wsOBSvc.GetOrderLatestTimestamp(_order, ts, false)

		var askKeys = make(map[interface{}]bool)
		var bidKeys = make(map[interface{}]bool)

		var bidsData = make([][]interface{}, 0)
		var asksData = make([][]interface{}, 0)
		var changeAsksRaw = make(map[string]float64)
		var changeBidsRaw = make(map[string]float64)
		if len(orderBook.Asks) > 0 {
			for _, ask := range orderBook.Asks {
				var askData []interface{}
				// check if data from last changeId is changed or is there new data incoming
				if val, ok := changeId.Asks[fmt.Sprintf("%f", ask.Price)]; ok {
					if val != ask.Amount {
						if ask.Amount == 0 {
							askData = append(askData, "delete")
						} else {
							askData = append(askData, "change")
							changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
						}
						askData = append(askData, ask.Price)
						askData = append(askData, ask.Amount)
						asksData = append(asksData, askData)
						askKeys[ask.Price] = true
					} else {
						changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
					}
				} else {
					if ask.Amount != 0 {
						askData = append(askData, "new")
						askData = append(askData, ask.Price)
						askData = append(askData, ask.Amount)
						asksData = append(asksData, askData)
						askKeys[ask.Price] = true
						changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
					}
				}
			}
		} else {
			asksData = make([][]interface{}, 0)
		}

		if len(orderBook.Bids) > 0 {
			for _, bid := range orderBook.Bids {
				var bidData []interface{}
				// check if data from last changeId is changed or is there new data incoming
				if val, ok := changeId.Bids[fmt.Sprintf("%f", bid.Price)]; ok {
					if val != bid.Amount {
						if bid.Amount == 0 {
							bidData = append(bidData, "delete")
						} else {
							bidData = append(bidData, "change")
							changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
						}
						bidData = append(bidData, bid.Price)
						bidData = append(bidData, bid.Amount)
						bidsData = append(bidsData, bidData)
						bidKeys[bid.Price] = true
					} else {
						changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
					}
				} else {
					if bid.Amount != 0 {
						bidData = append(bidData, "new")
						bidData = append(bidData, bid.Price)
						bidData = append(bidData, bid.Amount)
						bidsData = append(bidsData, bidData)
						bidKeys[bid.Price] = true
						changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
					}
				}
			}
		} else {
			bidsData = make([][]interface{}, 0)
		}
		// Check on cancelled orders
		for _, book := range books {
			for _, ask := range book.Asks {
				if _, ok := askKeys[ask.Price]; ok {
					continue
				}

				if amount, ok := changeId.Asks[fmt.Sprintf("%f", ask.Price)]; ok {
					if amount-ask.Amount == 0 {
						var askData []interface{}
						askData = append(askData, "delete")
						askData = append(askData, ask.Price)
						askData = append(askData, 0)
						asksData = append(asksData, askData)
						askKeys[ask.Price] = true
					}
				}
			}
			for _, bid := range book.Bids {
				if _, ok := bidKeys[bid.Price]; ok {
					continue
				}
				if amount, ok := changeId.Bids[fmt.Sprintf("%f", bid.Price)]; ok {
					if amount-bid.Amount == 0 {
						var bidData []interface{}
						bidData = append(bidData, "delete")
						bidData = append(bidData, bid.Price)
						bidData = append(bidData, 0)
						bidsData = append(bidsData, bidData)
						bidKeys[bid.Price] = true
					}
				}
			}
		}

		// Set new data into redis
		id := changeId.Id + 1
		changeIdNew = types.Change{
			Id:            id,
			IdPrev:        changeId.Id,
			Timestamp:     ts,
			TimestampPrev: changeId.Timestamp,
			Asks:          changeAsksRaw,
			Bids:          changeBidsRaw,
			Asks100:       changeId.Asks100,
			Bids100:       changeId.Bids100,
			AsksAgg:       changeId.AsksAgg,
			BidsAgg:       changeId.BidsAgg,
		}

		//convert changeIdNew to json
		jsonBytes, err := json.Marshal(changeIdNew)
		if err != nil {
			fmt.Println(err)
			return
		}
		svc.redis.Set("CHANGEID-"+_instrument, string(jsonBytes))

		go svc.HandleConsumeBookAgg(_instrument, *order, true, books)

		changeIdMutex.Lock()
		if _, ok := changeId100ms[_instrument]; !ok {
			changeId100ms[_instrument] = types.ChangeStruct{
				Id:             changeIdNew.Id,
				IdPrev:         changeIdNew.IdPrev,
				Status:         order.Status,
				Side:           order.Side,
				Price:          order.Price,
				Amount:         order.Amount,
				Amendments:     order.Amendments,
				CancelledBooks: books,
			}
			go svc.Handle100msInterval(_instrument)
		} else {
			changeId100ms[_instrument] = types.ChangeStruct{
				Id:             changeIdNew.Id,
				IdPrev:         changeIdNew.IdPrev,
				Status:         order.Status,
				Side:           order.Side,
				Price:          order.Price,
				Amount:         order.Amount,
				Amendments:     order.Amendments,
				CancelledBooks: books,
			}
		}
		changeIdMutex.Unlock()

		bookData := types.BookData{
			Type:           "change",
			Timestamp:      changeIdNew.Timestamp,
			InstrumentName: _instrument,
			ChangeId:       changeIdNew.Id,
			PrevChangeId:   changeIdNew.IdPrev,
			Bids:           bidsData,
			Asks:           asksData,
		}

		params := types.QuoteResponse{
			Channel: fmt.Sprintf("book.%s.raw", _instrument),
			Data:    bookData,
		}
		method := "subscription"
		broadcastId := fmt.Sprintf("%s-raw", _instrument)
		ws.GetBookSocket().BroadcastMessage(broadcastId, method, params)
	}
}

func (svc orderbookHandler) HandleConsumeBookAgg(_instrument string, order types.Order, isCancelledAll bool, cancelledBooks map[string]types.OrderbookMap) {
	_order := types.GetOrderBook{
		InstrumentName: _instrument,
		Underlying:     order.Underlying,
		ExpiryDate:     order.ExpiryDate,
		StrikePrice:    order.StrikePrice,
	}

	var changeId types.Change
	// Get change_id
	res, err := svc.redis.GetValue("CHANGEID-" + _instrument)
	if res == "" || err != nil {
		return
	} else {
		err = json.Unmarshal([]byte(res), &changeId)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Get data
	orderBook := svc.wsOBSvc.GetOrderLatestTimestampAgg(_order, changeId.Timestamp)

	var askKeys = make(map[interface{}]bool)
	var bidKeys = make(map[interface{}]bool)

	var bidsData = make([][]interface{}, 0)
	var asksData = make([][]interface{}, 0)
	var changeAsksRaw = make(map[string]float64)
	var changeBidsRaw = make(map[string]float64)

	if len(orderBook.Asks) > 0 {
		for _, ask := range orderBook.Asks {
			var askData []interface{}
			// check if data from last changeId is changed or is there new data incoming
			if val, ok := changeId.AsksAgg[fmt.Sprintf("%f", ask.Price)]; ok {
				if val != ask.Amount {
					if ask.Amount == 0 {
						askData = append(askData, "delete")
					} else {
						askData = append(askData, "change")
						changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
					}
					askData = append(askData, ask.Price)
					askData = append(askData, ask.Amount)
					asksData = append(asksData, askData)
					askKeys[ask.Price] = true
				} else {
					changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
				}
			} else {
				if ask.Amount != 0 {
					askData = append(askData, "new")
					askData = append(askData, ask.Price)
					askData = append(askData, ask.Amount)
					asksData = append(asksData, askData)
					askKeys[ask.Price] = true
					changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
				}
			}
		}
	} else {
		asksData = make([][]interface{}, 0)
	}

	if len(orderBook.Bids) > 0 {
		for _, bid := range orderBook.Bids {
			var bidData []interface{}
			// check if data from last changeId is changed or is there new data incoming
			if val, ok := changeId.BidsAgg[fmt.Sprintf("%f", bid.Price)]; ok {
				if val != bid.Amount {
					if bid.Amount == 0 {
						bidData = append(bidData, "delete")
					} else {
						bidData = append(bidData, "change")
						changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
					}
					bidData = append(bidData, bid.Price)
					bidData = append(bidData, bid.Amount)
					bidsData = append(bidsData, bidData)
					bidKeys[bid.Price] = true
				} else {
					changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
				}
			} else {
				if bid.Amount != 0 {
					bidData = append(bidData, "new")
					bidData = append(bidData, bid.Price)
					bidData = append(bidData, bid.Amount)
					bidsData = append(bidsData, bidData)
					bidKeys[bid.Price] = true
					changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
				}
			}
		}
	} else {
		bidsData = make([][]interface{}, 0)
	}

	if isCancelledAll {
		// Check on cancelled orders
		for _, book := range cancelledBooks {
			for _, ask := range book.Asks {
				if _, ok := askKeys[ask.Price]; ok {
					continue
				}
				if amount, ok := changeId.AsksAgg[fmt.Sprintf("%f", ask.Price)]; ok {
					if amount-ask.Amount == 0 {
						var askData []interface{}
						askData = append(askData, "delete")
						askData = append(askData, ask.Price)
						askData = append(askData, 0)
						asksData = append(asksData, askData)
						askKeys[ask.Price] = true
					}
				}
			}
			for _, bid := range book.Bids {
				if _, ok := bidKeys[bid.Price]; ok {
					continue
				}
				if amount, ok := changeId.BidsAgg[fmt.Sprintf("%f", bid.Price)]; ok {
					if amount-bid.Amount == 0 {
						var bidData []interface{}
						bidData = append(bidData, "delete")
						bidData = append(bidData, bid.Price)
						bidData = append(bidData, 0)
						bidsData = append(bidsData, bidData)
						bidKeys[bid.Price] = true
					}
				}

			}
		}
	} else {
		// Check on order cancel
		if order.Status == "CANCELLED" {
			// Check if price point deleted
			switch order.Side {
			case "BUY":
				if amount, ok := changeId.BidsAgg[fmt.Sprintf("%f", order.Price)]; ok {
					if amount-order.Amount == 0 {
						var bidData []interface{}
						bidData = append(bidData, "delete")
						bidData = append(bidData, order.Price)
						bidData = append(bidData, 0)
						bidsData = append(bidsData, bidData)
					}
				}
			case "SELL":
				if amount, ok := changeId.AsksAgg[fmt.Sprintf("%f", order.Price)]; ok {
					if amount-order.Amount == 0 {
						var askData []interface{}
						askData = append(askData, "delete")
						askData = append(askData, order.Price)
						askData = append(askData, 0)
						asksData = append(asksData, askData)
					}
				}
			}
		} else if order.Status == "FILLED" { // Check on filled order
			switch order.Side {
			case "BUY":
				if amount, ok := changeId.AsksAgg[fmt.Sprintf("%f", order.Price)]; ok {
					if amount-order.Amount == 0 {
						var askData []interface{}
						askData = append(askData, "delete")
						askData = append(askData, order.Price)
						askData = append(askData, 0)
						asksData = append(asksData, askData)
					}
				}
			case "SELL":
				if amount, ok := changeId.BidsAgg[fmt.Sprintf("%f", order.Price)]; ok {
					if amount-order.Amount == 0 {
						var bidData []interface{}
						bidData = append(bidData, "delete")
						bidData = append(bidData, order.Price)
						bidData = append(bidData, 0)
						bidsData = append(bidsData, bidData)
					}
				}
			}
		} else if len(order.Amendments) > 0 {
			// Check on order edit
			updated := order.Amendments[len(order.Amendments)-1].UpdatedFields
			switch order.Side {
			case "BUY":
				if val, ok := updated["price"]; ok {
					if _, ok := changeId.BidsAgg[fmt.Sprintf("%f", val.OldValue)]; ok {
						// check if old price point deleted
						if val.OldValue != order.Price {
							var bidData []interface{}
							bidData = append(bidData, "delete")
							bidData = append(bidData, val.OldValue)
							bidData = append(bidData, 0)
							bidsData = append(bidsData, bidData)
						}
					}
				}
			case "SELL":
				if val, ok := updated["price"]; ok {
					if _, ok := changeId.AsksAgg[fmt.Sprintf("%f", val.OldValue)]; ok {
						// check if old price point deleted
						if val.OldValue != order.Price {
							var askData []interface{}
							askData = append(askData, "delete")
							askData = append(askData, val.OldValue)
							askData = append(askData, 0)
							asksData = append(asksData, askData)
						}
					}
				}
			}
		}
	}

	// Set new data into redis
	changeIdNew := types.Change{
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

	//convert changeIdNew to json
	jsonBytes, err := json.Marshal(changeIdNew)
	if err != nil {
		fmt.Println(err)
		return
	}

	svc.redis.Set("CHANGEID-"+_instrument, string(jsonBytes))

	bookData := types.BookData{
		Type:           "change",
		Timestamp:      changeId.Timestamp,
		InstrumentName: _instrument,
		ChangeId:       changeId.Id,
		PrevChangeId:   changeId.IdPrev,
		Bids:           bidsData,
		Asks:           asksData,
	}

	params := types.QuoteResponse{
		Channel: fmt.Sprintf("book.%s.agg2", _instrument),
		Data:    bookData,
	}
	method := "subscription"
	id := fmt.Sprintf("%s-agg2", _instrument)
	ws.GetBookSocket().BroadcastMessage(id, method, params)
}

func (svc orderbookHandler) Handle100msInterval(instrument string) {
	// create new ticker on 100ms intervak
	ticker := time.NewTicker(100 * time.Millisecond)
	var changeIdLocalVar types.ChangeStruct

	// Creating channel
	tickerChan := make(chan bool)
	go func() {
		for {
			select {
			case <-tickerChan:
				return
			case <-ticker.C:
				// if there is no change no need to broadcast
				changeIdMutex.RLock()
				changeId100msLocal := changeId100ms[instrument]
				changeIdMutex.RUnlock()
				if changeIdLocalVar.Id != changeId100msLocal.Id {
					var prevId int
					if changeIdLocalVar.Id == 0 {
						res, err := svc.redis.GetValue("SNAPSHOTID-" + instrument)
						if res == "" || err != nil {
							changeIdLocalVar = changeId100msLocal
						} else {
							prevId, _ = strconv.Atoi(res)
						}
					} else {
						prevId = changeIdLocalVar.Id
					}
					changeIdLocalVar = changeId100msLocal

					substring := strings.Split(instrument, "-")
					_strikePrice, err := strconv.ParseFloat(substring[2], 64)
					if err != nil {
						fmt.Println(err)
						continue
					}
					_underlying := substring[0]
					_expiryDate := strings.ToUpper(substring[1])

					_order := types.GetOrderBook{
						InstrumentName: instrument,
						Underlying:     _underlying,
						ExpiryDate:     _expiryDate,
						StrikePrice:    _strikePrice,
					}

					ts := time.Now().UnixNano() / int64(time.Millisecond)
					var changeId types.Change
					// Get saved orderbook from redis
					res, err := svc.redis.GetValue("CHANGEID-" + instrument)
					if res == "" || err != nil {
						continue
					} else {
						err = json.Unmarshal([]byte(res), &changeId)
						if err != nil {
							fmt.Println(err)
							continue
						}
					}
					// Get latest data from db
					orderBook := svc.wsOBSvc.GetOrderLatestTimestamp(_order, ts, false)

					var askKeys = make(map[interface{}]bool)
					var bidKeys = make(map[interface{}]bool)

					var bidsData = make([][]interface{}, 0)
					var asksData = make([][]interface{}, 0)
					var changeAsksRaw = make(map[string]float64)
					var changeBidsRaw = make(map[string]float64)

					if len(orderBook.Asks) > 0 {
						for _, ask := range orderBook.Asks {
							var askData []interface{}
							// check if data from last changeId is changed or is there new data incoming
							if val, ok := changeId.Asks100[fmt.Sprintf("%f", ask.Price)]; ok {
								if val != ask.Amount {
									if ask.Amount == 0 {
										askData = append(askData, "delete")
									} else {
										askData = append(askData, "change")
										changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
									}
									askData = append(askData, ask.Price)
									askData = append(askData, ask.Amount)
									asksData = append(asksData, askData)
									askKeys[ask.Price] = true
								} else {
									changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
								}
							} else {
								if ask.Amount != 0 {
									askData = append(askData, "new")
									askData = append(askData, ask.Price)
									askData = append(askData, ask.Amount)
									asksData = append(asksData, askData)
									askKeys[ask.Price] = true
									changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
								}
							}
						}
					} else {
						asksData = make([][]interface{}, 0)
					}

					if len(orderBook.Bids) > 0 {
						for _, bid := range orderBook.Bids {
							var bidData []interface{}
							// check if data from last changeId is changed or is there new data incoming
							if val, ok := changeId.Bids100[fmt.Sprintf("%f", bid.Price)]; ok {
								if val != bid.Amount {
									if bid.Amount == 0 {
										bidData = append(bidData, "delete")
									} else {
										bidData = append(bidData, "change")
										changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
									}
									bidData = append(bidData, bid.Price)
									bidData = append(bidData, bid.Amount)
									bidsData = append(bidsData, bidData)
									bidKeys[bid.Price] = true
								} else {
									changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
								}
							} else {
								if bid.Amount != 0 {
									bidData = append(bidData, "new")
									bidData = append(bidData, bid.Price)
									bidData = append(bidData, bid.Amount)
									bidsData = append(bidsData, bidData)
									bidKeys[bid.Price] = true
									changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
								}
							}
						}
					} else {
						bidsData = make([][]interface{}, 0)
					}
					// Check on order cancel
					if len(changeIdLocalVar.CancelledBooks) > 0 {
						for _, book := range changeIdLocalVar.CancelledBooks {
							for _, ask := range book.Asks {
								if _, ok := askKeys[ask.Price]; ok {
									continue
								}
								if amount, ok := changeId.Asks100[fmt.Sprintf("%f", ask.Price)]; ok {
									if amount-ask.Amount == 0 {
										var askData []interface{}
										askData = append(askData, "delete")
										askData = append(askData, ask.Price)
										askData = append(askData, 0)
										asksData = append(asksData, askData)
										askKeys[ask.Price] = true
									}
								}
							}
							for _, bid := range book.Bids {
								if _, ok := bidKeys[bid.Price]; ok {
									continue
								}
								if amount, ok := changeId.Bids100[fmt.Sprintf("%f", bid.Price)]; ok {
									if amount-bid.Amount == 0 {
										var bidData []interface{}
										bidData = append(bidData, "delete")
										bidData = append(bidData, bid.Price)
										bidData = append(bidData, 0)
										bidsData = append(bidsData, bidData)
										bidKeys[bid.Price] = true
									}
								}
							}
						}
					} else {
						if changeIdLocalVar.Status == "CANCELLED" {
							// Check if price point deleted
							switch changeIdLocalVar.Side {
							case "BUY":
								if amount, ok := changeId.Bids100[fmt.Sprintf("%f", changeIdLocalVar.Price)]; ok {
									if amount-changeIdLocalVar.Amount == 0 {
										var bidData []interface{}
										bidData = append(bidData, "delete")
										bidData = append(bidData, changeIdLocalVar.Price)
										bidData = append(bidData, 0)
										bidsData = append(bidsData, bidData)
									}
								}
							case "SELL":
								if amount, ok := changeId.Asks100[fmt.Sprintf("%f", changeIdLocalVar.Price)]; ok {
									if amount-changeIdLocalVar.Amount == 0 {
										var askData []interface{}
										askData = append(askData, "delete")
										askData = append(askData, changeIdLocalVar.Price)
										askData = append(askData, 0)
										asksData = append(asksData, askData)
									}
								}
							}
						} else if changeIdLocalVar.Status == "FILLED" { // Check on filled order
							switch changeIdLocalVar.Side {
							case "BUY":
								if amount, ok := changeId.Asks100[fmt.Sprintf("%f", changeIdLocalVar.Price)]; ok {
									if amount-changeIdLocalVar.Amount == 0 {
										var askData []interface{}
										askData = append(askData, "delete")
										askData = append(askData, changeIdLocalVar.Price)
										askData = append(askData, 0)
										asksData = append(asksData, askData)
									}
								}
							case "SELL":
								if amount, ok := changeId.Bids100[fmt.Sprintf("%f", changeIdLocalVar.Price)]; ok {
									if amount-changeIdLocalVar.Amount == 0 {
										var bidData []interface{}
										bidData = append(bidData, "delete")
										bidData = append(bidData, changeIdLocalVar.Price)
										bidData = append(bidData, 0)
										bidsData = append(bidsData, bidData)
									}
								}
							}
						} else if len(changeIdLocalVar.Amendments) > 0 {
							// Check on order edit
							updated := changeIdLocalVar.Amendments[len(changeIdLocalVar.Amendments)-1].UpdatedFields
							switch changeIdLocalVar.Side {
							case "BUY":
								if val, ok := updated["price"]; ok {
									if _, ok := changeId.Bids100[fmt.Sprintf("%f", val.OldValue)]; ok {
										// check if old price point deleted

										if val.OldValue != changeIdLocalVar.Price {
											var bidData []interface{}
											bidData = append(bidData, "delete")
											bidData = append(bidData, val.OldValue)
											bidData = append(bidData, 0)
											bidsData = append(bidsData, bidData)
										}
									}
								}
							case "SELL":
								if val, ok := updated["price"]; ok {
									if _, ok := changeId.Asks100[fmt.Sprintf("%f", val.OldValue)]; ok {
										// check if old price point deleted
										if val.OldValue != changeIdLocalVar.Price {
											var askData []interface{}
											askData = append(askData, "delete")
											askData = append(askData, val.OldValue)
											askData = append(askData, 0)
											asksData = append(asksData, askData)
										}
									}
								}
							}
						}
					}

					// Set new data into redis
					changeIdNew := types.Change{
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

					//convert changeIdNew to json
					jsonBytes, err := json.Marshal(changeIdNew)
					if err != nil {
						fmt.Println(err)
						return
					}

					svc.redis.Set("CHANGEID-"+instrument, string(jsonBytes))

					bookData := types.BookData{
						Type:           "change",
						Timestamp:      changeIdNew.Timestamp,
						InstrumentName: instrument,
						ChangeId:       changeIdNew.Id,
						PrevChangeId:   prevId,
						Bids:           bidsData,
						Asks:           asksData,
					}

					params := types.QuoteResponse{
						Channel: fmt.Sprintf("book.%s.100ms", instrument),
						Data:    bookData,
					}
					method := "subscription"
					broadcastId := fmt.Sprintf("%s-100ms", instrument)
					ordermatch.OnMarketDataUpdate(instrument, bookData)
					ws.GetBookSocket().BroadcastMessage(broadcastId, method, params)
				}
			}
		}
	}()
}
