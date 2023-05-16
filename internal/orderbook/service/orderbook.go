package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"

	_orderbookType "gateway/internal/orderbook/types"
	wsService "gateway/internal/ws/service"

	"gateway/internal/orderbook/types"

	"gateway/pkg/redis"
	"gateway/pkg/ws"
)

type orderbookHandler struct {
	redis   *redis.RedisConnectionPool
	wsOBSvc wsService.IwsOrderbookService
}

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

func (svc orderbookHandler) HandleConsumeBook(msg *sarama.ConsumerMessage) {
	var order _orderbookType.Order

	err := json.Unmarshal(msg.Value, &order)
	if err != nil {
		fmt.Println(err)
		return
	}
	_instrument := order.Underlying + "-" + order.ExpiryDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0])

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
	orderBook := svc.wsOBSvc.GetOrderLatestTimestamp(_order, ts)

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
					askData = append(askData, "change")
					askData = append(askData, ask.Price)
					askData = append(askData, ask.Amount)
					bidsData = append(bidsData, askData)
				}
			} else {
				askData = append(askData, "new")
				askData = append(askData, ask.Price)
				askData = append(askData, ask.Amount)
				asksData = append(asksData, askData)
			}
			changeAsksRaw[fmt.Sprintf("%f", ask.Price)] = ask.Amount
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
					bidData = append(bidData, "change")
					bidData = append(bidData, bid.Price)
					bidData = append(bidData, bid.Amount)
					bidsData = append(bidsData, bidData)
				}
			} else {
				bidData = append(bidData, "new")
				bidData = append(bidData, bid.Price)
				bidData = append(bidData, bid.Amount)
				bidsData = append(bidsData, bidData)
			}
			changeBidsRaw[fmt.Sprintf("%f", bid.Price)] = bid.Amount
		}
	} else {
		bidsData = make([][]interface{}, 0)
	}
	// Check on order cancel
	if order.Status == "CANCELLED" {
		// Check if price point deleted
		if amount, ok := changeId.Bids[fmt.Sprintf("%f", order.Price)]; ok {
			if amount-order.Amount == 0 {
				switch order.Side {
				case "BUY":
					var bidData []interface{}
					bidData = append(bidData, "delete")
					bidData = append(bidData, order.Price)
					bidData = append(bidData, 0)
					bidsData = append(bidsData, bidData)
				case "SELL":
					var askData []interface{}
					askData = append(askData, "delete")
					askData = append(askData, order.Price)
					askData = append(askData, 0)
					asksData = append(asksData, askData)
				}
			}
		}
	} else if len(order.Amendments) > 0 {
		// Check on order edit
		updated := order.Amendments[len(order.Amendments)-1].UpdatedFields
		switch order.Side {
		case "BUY":
			if val, ok := updated["price"]; ok {
				if amount, ok := changeId.Bids[fmt.Sprintf("%f", val.OldValue)]; ok {
					// check if old price point deleted
					if amount-order.Amount == 0 {
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
				if amount, ok := changeId.Asks[fmt.Sprintf("%f", val.OldValue)]; ok {
					// check if old price point deleted
					if amount-order.Amount == 0 {
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
	}

	//convert changeIdNew to json
	jsonBytes, err := json.Marshal(changeIdNew)
	if err != nil {
		fmt.Println(err)
		return
	}
	svc.redis.Set("CHANGEID-"+_instrument, string(jsonBytes))

	go svc.HandleConsumeBookAgg(msg)

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

func (svc orderbookHandler) HandleConsumeBookAgg(msg *sarama.ConsumerMessage) {
	var order _orderbookType.Order

	err := json.Unmarshal(msg.Value, &order)
	if err != nil {
		fmt.Println(err)
		return
	}
	_instrument := order.Underlying + "-" + order.ExpiryDate + "-" + fmt.Sprintf("%.0f", order.StrikePrice) + "-" + string(order.Contracts[0])

	_order := types.GetOrderBook{
		InstrumentName: _instrument,
		Underlying:     order.Underlying,
		ExpiryDate:     order.ExpiryDate,
		StrikePrice:    order.StrikePrice,
	}

	var status string
	if order.Status == "CANCELLED" {
		status = "delete"
	} else if len(order.Amendments) > 0 {
		status = "change"
	} else {
		status = "new"
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

	var bidsData [][]interface{}
	var asksData [][]interface{}
	if len(orderBook.Asks) > 0 {
		for _, ask := range orderBook.Asks {
			var askData []interface{}
			askData = append(askData, status)
			askData = append(askData, ask.Amount)
			askData = append(askData, ask.Price)
			asksData = append(asksData, askData)
		}
	} else {
		asksData = make([][]interface{}, 0)
	}
	if len(orderBook.Bids) > 0 {
		for _, bid := range orderBook.Bids {
			var bidData []interface{}
			bidData = append(bidData, status)
			bidData = append(bidData, bid.Amount)
			bidData = append(bidData, bid.Price)
			bidsData = append(bidsData, bidData)
		}
	} else {
		bidsData = make([][]interface{}, 0)
	}

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
	// to do
}
