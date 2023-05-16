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

	var status string
	if order.Status == "CANCELLED" {
		status = "delete"
	} else if len(order.Amendments) > 0 {
		status = "change"
	} else {
		status = "new"
	}

	ts := time.Now().UnixNano() / int64(time.Millisecond)
	var changeId types.Change
	var changeIdNew types.Change
	// Get change_id
	res, err := svc.redis.GetValue("CHANGEID-" + _instrument)
	if res == "" || err != nil {
		// Set initial data if null
		changeIdNew = types.Change{
			Id:        1,
			Timestamp: ts,
		}

		//convert changeIdNew to json
		jsonBytes, err := json.Marshal(changeIdNew)
		if err != nil {
			fmt.Println(err)
			return
		}
		svc.redis.Set("CHANGEID-"+_instrument, string(jsonBytes))
		changeId = types.Change{
			Id:        1,
			Timestamp: ts,
		}
	} else {
		err = json.Unmarshal([]byte(res), &changeId)
		if err != nil {
			fmt.Println(err)
			return
		}
		// Set new data
		id := changeId.Id + 1
		changeIdNew = types.Change{
			Id:            id,
			IdPrev:        changeId.Id,
			Timestamp:     ts,
			TimestampPrev: changeId.Timestamp,
		}

		//convert changeIdNew to json
		jsonBytes, err := json.Marshal(changeIdNew)
		if err != nil {
			fmt.Println(err)
			return
		}
		svc.redis.Set("CHANGEID-"+_instrument, string(jsonBytes))
	}
	go svc.HandleConsumeBookAgg(msg)

	// Get data
	orderBook := svc.wsOBSvc.GetOrderLatestTimestamp(_order, changeId.Timestamp, ts)

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
	id := fmt.Sprintf("%s-raw", _instrument)
	ws.GetBookSocket().BroadcastMessage(id, method, params)
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
	orderBook := svc.wsOBSvc.GetOrderLatestTimestampAgg(_order, changeId.TimestampPrev, changeId.Timestamp)

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
