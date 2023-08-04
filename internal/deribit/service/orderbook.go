package service

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/deribit/model"
	_engineTypes "gateway/internal/engine/types"
	_orderbookTypes "gateway/internal/orderbook/types"

	"gateway/pkg/memdb"
	"gateway/pkg/utils"
	"strings"
	"time"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
)

func (svc deribitService) GetOrderBook(ctx context.Context, data model.DeribitGetOrderBookRequest) *model.DeribitGetOrderBookResponse {
	instruments, _ := utils.ParseInstruments(data.InstrumentName, false)

	user, _, err := memdb.MDBFindUserById(data.UserId)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return nil
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

	results := model.DeribitGetOrderBookResponse{
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
		Stats: model.OrderBookStats{
			High:        orderBookValue.HighestPrice,
			Low:         orderBookValue.LowestPrice,
			PriceChange: orderBookValue.PriceChange,
			Volume:      orderBookValue.VolumeAmount,
		},
		Greeks: model.OrderBookGreek{
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

	_getSettlementPrice := svc.settlementPriceRepo.GetLatestSettlementPrice(
		_order.Underlying,
		_order.ExpiryDate,
	)
	if len(_getSettlementPrice) > 0 {
		results.SettlementPrice = &_getSettlementPrice[0].Price
	}

	return &results
}

func (svc deribitService) GetDataQuote(order _orderbookTypes.GetOrderBook) (_orderbookTypes.QuoteMessage, _orderbookTypes.Orderbook) {

	// Get initial data
	_getOrderBook := svc.orderRepo.GetOrderBook(order)

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

func (svc deribitService) GetIndexPrice(ctx context.Context, data model.DeribitGetIndexPriceRequest) model.DeribitGetIndexPriceResponse {
	var indexPrice float64

	_getIndexPrice := svc.rawPriceRepo.GetIndexPrice(data.IndexName)
	if len(_getIndexPrice) > 0 {
		indexPrice = float64(_getIndexPrice[0].Price)
	} else {
		indexPrice = float64(0)
	}
	result := model.DeribitGetIndexPriceResponse{
		IndexPrice: indexPrice,
	}
	return result
}

func (svc deribitService) GetDeliveryPrices(ctx context.Context, request model.DeliveryPricesRequest) model.DeliveryPricesResponse {
	_deliveryPrice, err := svc.settlementPriceRepo.GetDeliveryPrice(request)
	if err != nil {
		fmt.Println(err)
	}

	jsonBytes, err := json.Marshal(_deliveryPrice)
	if err != nil {
		fmt.Println(err)
	}

	var deliveryPrice model.DeliveryPricesResponse
	err = json.Unmarshal([]byte(jsonBytes), &deliveryPrice)
	if err != nil {
		fmt.Println(err)
	}

	return deliveryPrice
}

func (svc deribitService) GetDataOrderBook(_order _orderbookTypes.GetOrderBook, dataQuote _orderbookTypes.QuoteMessage) (_orderbookTypes.OrderBookData, []*_engineTypes.RawPrice, _orderbookTypes.MarkData) {
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
	_getLastTrades := svc.tradeRepo.GetLastTrades(_order)
	_lastPrice := 0.0
	if len(_getLastTrades) > 0 {
		_lastPrice = _getLastTrades[len(_getLastTrades)-1].Price
	}

	_getHigestTrade := svc.tradeRepo.GetHighLowTrades(_order, -1)
	_hightPrice := 0.0
	if len(_getHigestTrade) > 0 {
		_hightPrice = _getHigestTrade[0].Price
	}

	_getLowestTrade := svc.tradeRepo.GetHighLowTrades(_order, 1)
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

	_get24HoursTrade := svc.tradeRepo.Get24HoursTrades(_order)
	_priceChange := 0.0
	if len(_get24HoursTrade) > 0 {
		_firstTrade := _get24HoursTrade[0].Price
		_lastTrade := _get24HoursTrade[len(_get24HoursTrade)-1].Price

		//calculate price change with percentage
		_priceChange = (_lastTrade - _firstTrade) / _firstTrade * 100
	}

	//TODO query to get Underlying Price
	var underlyingPrice float64
	_getIndexPrice := svc.rawPriceRepo.GetLatestIndexPrice(_order)
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
	_getImpliedsAsk := svc.tradeRepo.GetImpliedVolatility(float64(dataQuote.BestAskAmount), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getImpliedsBid := svc.tradeRepo.GetImpliedVolatility(float64(dataQuote.BestBidAmount), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))

	//TODO query to get all greeks
	_getImpliedsVolatility := svc.tradeRepo.GetImpliedVolatility(float64(_lastPrice), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksDelta := svc.tradeRepo.GetGreeks("delta", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksVega := svc.tradeRepo.GetGreeks("vega", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksGamma := svc.tradeRepo.GetGreeks("gamma", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksTetha := svc.tradeRepo.GetGreeks("tetha", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	_getGreeksRho := svc.tradeRepo.GetGreeks("rho", float64(_getImpliedsVolatility), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))

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
		markData.MarkIv = svc.tradeRepo.GetImpliedVolatility(float64(markData.MarkPrice), optionPrice, float64(underlyingPrice), float64(_order.StrikePrice), float64(dateValue))
	}

	return value, _getIndexPrice, markData
}
