package repositories

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	_deribitModel "gateway/internal/deribit/model"
	_engineType "gateway/internal/engine/types"
	_orderbookType "gateway/internal/orderbook/types"
	_tradeType "gateway/internal/repositories/types"

	Greeks "git.devucc.name/dependencies/utilities/helper/greeks"
	IV "git.devucc.name/dependencies/utilities/helper/implied_volatility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TradeRepository struct {
	collection *mongo.Collection
}

func NewTradeRepository(db Database) *TradeRepository {
	collection := db.InitCollection("trades")
	return &TradeRepository{collection}
}

func (r TradeRepository) FilterTradesData(data _deribitModel.DeribitGetLastTradesByInstrumentRequest) []*_engineType.Trade {
	fmt.Println(data.StartSeq, " - type : ", reflect.TypeOf(data.StartSeq))

	// Querry for Instrument Name
	str := data.InstrumentName
	components := strings.Split(str, "-")

	underlying := components[0]
	expiryDate := components[1]
	strikePrice := components[2]
	contracts := components[3]

	switch contracts {
	case "C":
		contracts = "CALL"
	case "P":
		contracts = "PUT"
	}

	strikePriceFloat, _ := strconv.ParseFloat(strikePrice, 64)

	filter := bson.M{
		"underlying":  underlying,
		"expiryDate":  expiryDate,
		"strikePrice": strikePriceFloat,
		"contracts":   contracts,
	}

	// Querry for Sequence
	if data.StartSeq != 0 && data.EndSeq != 0 {
		filter["tradeSequence"] = bson.M{
			"$gte": data.StartSeq,
			"$lte": data.EndSeq,
		}
	} else {
		if data.StartSeq != 0 {
			filter["tradeSequence"] = bson.M{"$gte": data.StartSeq}
		}
		if data.EndSeq != 0 {
			filter["tradeSequence"] = bson.M{"$lte": data.EndSeq}
		}
	}

	// Querry for Time Stamp
	if !data.StartTimestamp.IsZero() && !data.EndTimestamp.IsZero() {
		filter["createdAt"] = bson.M{
			"$gte": data.StartTimestamp,
			"$lte": data.EndTimestamp,
		}
	} else {
		if !data.StartTimestamp.IsZero() {
			filter["createdAt"] = bson.M{"$gte": data.StartTimestamp}
		}
		if !data.EndTimestamp.IsZero() {
			filter["createdAt"] = bson.M{"$lte": data.EndTimestamp}
		}
	}

	// Querry for Instrument End Time Stamp

	// Querry for Count
	limit := int64(data.Count)
	if limit == 0 {
		limit = 3
	}

	findOptions := options.Find()
	findOptions.SetLimit(limit)

	sortOrder := -1 // Default sort order is descending

	switch data.Sorting {
	case "asc":
		sortOrder = 1
	case "desc":
		sortOrder = -1
	}

	if data.Sorting == "" {
		// Set your default sort order here
		sortOrder = -1
	}

	findOptions.SetSort(bson.M{"createdAt": sortOrder})

	cursor, err := r.collection.Find(context.Background(), filter, findOptions)
	if err != nil {
		return nil
	}

	defer cursor.Close(context.Background())

	Trades := []*_engineType.Trade{}

	err = cursor.All(context.Background(), &Trades)
	if err != nil {
		return nil
	}

	return Trades
}

func (r TradeRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*_engineType.Trade, error) {
	options := options.FindOptions{
		MaxTime: &defaultTimeout,
	}

	if offset >= 0 {
		options.SetSkip(offset)
	}

	if limit >= 0 {
		options.SetLimit(limit)
	}

	if sort != nil {
		options.SetSort(sort)
	}

	if filter == nil {
		filter = bson.M{}
	}

	cursor, err := r.collection.Find(context.Background(), filter, &options)
	if err != nil {
		return nil, err
	}

	defer cursor.Close(context.Background())

	Trades := []*_engineType.Trade{}

	err = cursor.All(context.Background(), &Trades)
	if err != nil {
		return nil, err
	}

	return Trades, nil
}

func (r TradeRepository) FindUserTradesByInstrument(
	instrument string,
	sort string,
	count int,
	userId string,
) (result _deribitModel.DeribitGetUserTradesByInstrumentsResponse, err error) {
	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	_string := instrument
	substring := strings.Split(_string, "-")

	_strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		fmt.Println(err)
	}
	_underlying := substring[0]
	_expiryDate := strings.ToUpper(substring[1])

	query := bson.A{
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "taker.orderId"},
					{"foreignField", "_id"},
					{"as", "takerOrder"},
				},
			},
		},
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "maker.orderId"},
					{"foreignField", "_id"},
					{"as", "makerOrder"},
				},
			},
		},
		bson.D{
			{"$match",
				bson.D{
					{"underlying", _underlying},
					{"strikePrice", _strikePrice},
					{"expiryDate", _expiryDate},
					{"$or",
						bson.A{
							bson.D{{"taker.userId", userId}},
							bson.D{{"maker.userId", userId}},
						},
					},
				},
			},
		},
		bson.D{
			{"$project",
				bson.D{
					{"InstrumentName", bson.M{"$concat": bson.A{
						bson.D{
							{"$convert", bson.D{
								{"input", "$underlying"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$expiryDate"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$strikePrice"},
								{"to", "string"},
							}}},
						"-",
						bson.M{"$substr": bson.A{"$contracts", 0, 1}},
					}}},
					{"amount", "$amount"},
					{"direction", "$side"},
					{"label",
						bson.D{
							{"$cond",
								bson.A{
									"$taker.userId" == userId,
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrder.label",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.label",
												0,
											},
										},
									},
								},
							},
						},
					},
					{"order_id",
						bson.D{
							{"$cond",
								bson.A{
									"$taker.userId" == userId,
									"$taker.orderId",
									"$maker.orderId",
								},
							},
						},
					},
					{"order_type",
						bson.D{
							{"$cond",
								bson.A{
									"$taker.userId" == userId,
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrder.type",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.type",
												0,
											},
										},
									},
								},
							},
						},
					},
					{"price", "$price"},
					{"state",
						bson.D{
							{"$cond",
								bson.A{
									"$takerId" == userId,
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrderId.status",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.status",
												0,
											},
										},
									},
								},
							},
						},
					},
					{"timestamp", bson.M{"$toLong": "$createdAt"}},
					{"strikePrice", "$strikePrice"},
					{"tickDirection", "$tickDirection"},
					{"tradeSequence", "$tradeSequence"},
					{"indexPrice", "$indexPrice"},
				},
			},
		},
		bson.D{
			{"$facet",
				bson.D{
					{"trades",
						bson.A{
							bson.D{{"$skip", 0}},
							bson.D{{"$limit", count}},
						},
					},
					{"total",
						bson.A{
							bson.D{{"$count", "count"}},
						},
					},
				},
			},
		},
	}

	var cursor *mongo.Cursor
	cursor, err = r.collection.Aggregate(context.Background(), query, &options)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cursor.Close(context.Background())

	var res _tradeType.UserTradesByInstrumentResult
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&res); err != nil {
			fmt.Println(err)
			return
		}
		for _, trade := range res.Trades {
			trade.Api = true
		}
	}

	result.Trades = res.Trades
	if len(res.Total) > 0 {
		result.HasMore = res.Total[0].Count > int64(count)
	}

	return result, nil
}

func (r TradeRepository) FindUserTradesById(
	instrument string,
	userId []interface{},
	orderId []interface{},
) (result _tradeType.UserTradesByInstrumentResult, err error) {
	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	_string := instrument
	substring := strings.Split(_string, "-")

	_strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		fmt.Println(err)
	}
	_underlying := substring[0]
	_expiryDate := strings.ToUpper(substring[1])

	query := bson.A{
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "taker.orderId"},
					{"foreignField", "_id"},
					{"as", "takerOrder"},
				},
			},
		},
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "maker.orderId"},
					{"foreignField", "_id"},
					{"as", "makerOrder"},
				},
			},
		},
		bson.D{
			{"$match",
				bson.D{
					{"_id", bson.M{"$in": orderId}},
					{"underlying", _underlying},
					{"strikePrice", _strikePrice},
					{"expiryDate", _expiryDate},
					{"$or",
						bson.A{
							bson.M{"taker.userId": bson.M{"$in": userId}},
							bson.M{"maker.userId": bson.M{"$in": userId}},
						},
					},
				},
			},
		},
		bson.D{
			{"$project",
				bson.D{
					{"InstrumentName", bson.M{"$concat": bson.A{
						bson.D{
							{"$convert", bson.D{
								{"input", "$underlying"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$expiryDate"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$strikePrice"},
								{"to", "string"},
							}}},
						"-",
						bson.M{"$substr": bson.A{"$contracts", 0, 1}},
					}}},
					{"amount", "$amount"},
					{"direction", "$side"},
					{"label",
						bson.D{
							{"$cond",
								bson.A{
									bson.M{"$in": bson.A{"$userId", userId}},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrder.label",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.label",
												0,
											},
										},
									},
								},
							},
						},
					},
					{"order_id",
						bson.D{
							{"$cond",
								bson.A{
									bson.M{"$in": bson.A{"$userId", userId}},
									"$taker.orderId",
									"$maker.orderId",
								},
							},
						},
					},
					{"order_type",
						bson.D{
							{"$cond",
								bson.A{
									bson.M{"$in": bson.A{"$userId", userId}},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrder.type",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.type",
												0,
											},
										},
									},
								},
							},
						},
					},
					{"price", "$price"},
					{"state",
						bson.D{
							{"$cond",
								bson.A{
									bson.M{"$in": bson.A{"$userId", userId}},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$takerOrderId.status",
												0,
											},
										},
									},
									bson.D{
										{"$arrayElemAt",
											bson.A{
												"$makerOrder.status",
												0,
											},
										},
									},
								},
							},
						},
					},
					{"timestamp", bson.M{"$toLong": "$createdAt"}},
					{"strikePrice", "$strikePrice"},
					{"tickDirection", "$tickDirection"},
					{"tradeSequence", "$tradeSequence"},
					{"indexPrice", "$indexPrice"},
				},
			},
		},
		bson.D{
			{"$facet",
				bson.D{
					{"trades",
						bson.A{
							bson.D{{"$skip", 0}},
						},
					},
					{"total",
						bson.A{
							bson.D{{"$count", "count"}},
						},
					},
				},
			},
		},
	}

	var cursor *mongo.Cursor
	cursor, err = r.collection.Aggregate(context.Background(), query, &options)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cursor.Close(context.Background())

	var res _tradeType.UserTradesByInstrumentResult
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&res); err != nil {
			fmt.Println(err)
			return
		}
		for _, trade := range res.Trades {
			trade.Api = true
		}
	}

	result.Trades = res.Trades

	return result, nil
}

func (r TradeRepository) FindTradesByInstrument(
	instrument string,
	orderId []interface{},
) (result _tradeType.UserTradesByInstrumentResult, err error) {
	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	_string := instrument
	substring := strings.Split(_string, "-")

	_strikePrice, err := strconv.ParseFloat(substring[2], 64)
	if err != nil {
		fmt.Println(err)
	}
	_underlying := substring[0]
	_expiryDate := strings.ToUpper(substring[1])

	query := bson.A{
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "taker.orderId"},
					{"foreignField", "_id"},
					{"as", "takerOrder"},
				},
			},
		},
		bson.D{
			{"$lookup",
				bson.D{
					{"from", "orders"},
					{"localField", "maker.orderId"},
					{"foreignField", "_id"},
					{"as", "makerOrder"},
				},
			},
		},
		bson.D{
			{"$match",
				bson.D{
					{"_id", bson.M{"$in": orderId}},
					{"underlying", _underlying},
					{"strikePrice", _strikePrice},
					{"expiryDate", _expiryDate},
				},
			},
		},
		bson.D{
			{"$project",
				bson.D{
					{"InstrumentName", bson.M{"$concat": bson.A{
						bson.D{
							{"$convert", bson.D{
								{"input", "$underlying"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$expiryDate"},
								{"to", "string"},
							}}},
						"-",
						bson.D{
							{"$convert", bson.D{
								{"input", "$strikePrice"},
								{"to", "string"},
							}}},
						"-",
						bson.M{"$substr": bson.A{"$contracts", 0, 1}},
					}}},
					{"amount", "$amount"},
					{"direction", "$side"},
					{"label",
						bson.D{
							{"$arrayElemAt",
								bson.A{
									"$takerOrder.label",
									0,
								},
							},
						},
					},
					{"order_id",
						bson.D{
							{"$ifNull",
								bson.A{
									"$taker.orderId",
									"$taker.orderId",
									"$maker.orderId",
								},
							},
						},
					},
					{"order_type",
						bson.D{
							{"$arrayElemAt",
								bson.A{
									"$takerOrder.type",
									0,
								},
							},
						},
					},
					{"price", "$price"},
					{"state",
						bson.D{
							{"$arrayElemAt",
								bson.A{
									"$takerOrder.status",
									0,
								},
							},
						},
					},
					{"timestamp", bson.M{"$toLong": "$createdAt"}},
					{"strikePrice", "$strikePrice"},
					{"tickDirection", "$tickDirection"},
					{"tradeSequence", "$tradeSequence"},
					{"indexPrice", "$indexPrice"},
				},
			},
		},
		bson.D{
			{"$facet",
				bson.D{
					{"trades",
						bson.A{
							bson.D{{"$skip", 0}},
						},
					},
					{"total",
						bson.A{
							bson.D{{"$count", "count"}},
						},
					},
				},
			},
		},
	}

	var cursor *mongo.Cursor
	cursor, err = r.collection.Aggregate(context.Background(), query, &options)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cursor.Close(context.Background())

	var res _tradeType.UserTradesByInstrumentResult
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&res); err != nil {
			fmt.Println(err)
			return
		}
		for _, trade := range res.Trades {
			trade.Api = true
		}
	}

	result.Trades = res.Trades

	return result, nil
}

func (r TradeRepository) GetPriceAvg(underlying, expiryDate, contracts string, strikePrice float64) (price float64, err error) {
	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	query := bson.A{
		bson.D{
			{"$match",
				bson.D{
					{"underlying", underlying},
					{"strikePrice", strikePrice},
					{"expiryDate", expiryDate},
					{"contracts", contracts},
				},
			},
		},
		bson.D{
			{"$group",
				bson.D{
					{"_id", primitive.Null{}},
					{"price", bson.D{{"$avg", "$price"}}},
				},
			},
		},
		bson.D{
			{"$unset",
				bson.A{
					"_id",
				},
			},
		},
	}

	var cursor *mongo.Cursor
	cursor, err = r.collection.Aggregate(context.Background(), query, &options)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cursor.Close(context.Background())

	var avgPrice map[string]interface{}
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&avgPrice); err != nil {
			fmt.Println(err)
			return
		}
	}

	if val, ok := avgPrice["price"]; ok {
		price = val.(float64)
	}

	return
}

func (r TradeRepository) GetLastTrades(o _orderbookType.GetOrderBook) []*_engineType.Trade {
	tradesQuery := bson.M{
		"underlying":  o.Underlying,
		"strikePrice": o.StrikePrice,
		"expiryDate":  o.ExpiryDate,
	}
	tradesSort := bson.M{
		"createdAt": 1,
	}

	trades, err := r.Find(tradesQuery, tradesSort, 0, -1)
	if err != nil {
		panic(err)
	}

	return trades
}

func (r TradeRepository) GetHighLowTrades(o _orderbookType.GetOrderBook, t int) []*_engineType.Trade {
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

	trades, err := r.Find(tradesQuery, tradesSort, 0, -1)
	if err != nil {
		panic(err)
	}

	return trades
}

func (r TradeRepository) Get24HoursTrades(o _orderbookType.GetOrderBook) []*_engineType.Trade {
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

	trades, err := r.Find(tradesQuery, tradesSort, 0, -1)
	if err != nil {
		panic(err)
	}

	return trades
}

func (r TradeRepository) GetImpliedVolatility(marketPrice float64, optionPrice string, underlying float64, strikePrice float64, timeeXP float64) float64 {
	expectedCost := marketPrice // The market price of the option
	s := underlying             // Current price of the underlying
	k := strikePrice            // Strike price
	t := timeeXP                // Time to expiration in years
	ar := 0.05                  // Annual risk-free interest rate as a decimal
	callPut := optionPrice      // Type of option priced - "call" or "put"
	estimate := 0.1             // An initial estimate of implied volatility

	impliedData := IV.ImpliedVolatility(expectedCost, s, k, t, ar, callPut, estimate)

	return impliedData
}

func (r TradeRepository) GetGreeks(types string, impliedVolatily float64, optionPrice string, underlying float64, strikePrice float64, timeeXP float64) float64 {
	var delta float64

	if types == "delta" {
		s := underlying        // underlying price
		k := strikePrice       // strike price
		t := timeeXP           // time to maturity
		v := impliedVolatily   // volatility
		ar := 0.0015           // risk-free interest rate
		callPut := optionPrice // call or put

		delta = Greeks.GetDelta(s, k, t, v, ar, callPut)
	} else if types == "vega" {
		s := underlying      // underlying price
		k := strikePrice     // strike price
		t := timeeXP         // time to maturity
		v := impliedVolatily // volatility
		r := 0.0015          // risk-free interest rate

		delta = Greeks.GetVega(s, k, t, v, r)
	} else if types == "gamma" {
		s := underlying      // underlying price
		k := strikePrice     // strike price
		t := timeeXP         // time to maturity
		v := impliedVolatily // volatility
		r := 0.0015          // risk-free interest rate

		delta = Greeks.GetGamma(s, k, t, v, r)
	} else if types == "tetha" {
		s := underlying        // underlying price
		k := strikePrice       // strike price
		t := timeeXP           // time to maturity
		v := impliedVolatily   // volatility
		r := 0.0015            // risk-free interest rate
		callPut := optionPrice // Type of option priced - "call" or "put"
		scale := 365.0         // You can set the scale to a value like 252 (trading days per year), by default is 365

		delta = Greeks.GetTheta(s, k, t, v, r, callPut, scale)
	} else {
		s := underlying        // underlying price
		k := strikePrice       // strike price
		t := timeeXP           // time to maturity
		v := impliedVolatily   // volatility
		r := 0.0015            // risk-free interest rate
		callPut := optionPrice // Type of option priced - "call" or "put"
		scale := 100.0         // You can set the scale to a value like 10000 (rho per 0.01%, or 1BP, change in the risk-free interest rate), by default is 100

		delta = Greeks.GetRho(s, k, t, v, r, callPut, scale)
	}

	return delta
}

func (r TradeRepository) FilterUserTradesByOrder(userId string, orderId string) (result _deribitModel.DeribitGetUserTradesByOrderResponse, err error) {
	options := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	objectID, err := primitive.ObjectIDFromHex(orderId)
	if err != nil {
		fmt.Println(err)
		return
	}

	lookupTakerOrderStage := bson.D{
		{"$lookup",
			bson.D{
				{"from", "orders"},
				{"localField", "taker.orderId"},
				{"foreignField", "_id"},
				{"as", "takerOrder"},
			},
		},
	}

	lookupMakerOrderStage := bson.D{
		{"$lookup",
			bson.D{
				{"from", "orders"},
				{"localField", "maker.orderId"},
				{"foreignField", "_id"},
				{"as", "makerOrder"},
			},
		},
	}

	matchStage := bson.D{
		{"$match",
			bson.D{
				{"$or",
					bson.A{
						bson.D{{"taker.userId", userId}, {"taker.orderId", objectID}},
						bson.D{{"maker.userId", userId}, {"maker.orderId", objectID}},
					},
				},
			},
		},
	}

	projectStage := bson.D{
		{"$project",
			bson.D{
				{"InstrumentName", bson.M{"$concat": bson.A{
					bson.D{
						{"$convert", bson.D{
							{"input", "$underlying"},
							{"to", "string"},
						}}},
					"-",
					bson.D{
						{"$convert", bson.D{
							{"input", "$expiryDate"},
							{"to", "string"},
						}}},
					"-",
					bson.D{
						{"$convert", bson.D{
							{"input", "$strikePrice"},
							{"to", "string"},
						}}},
					"-",
					bson.M{"$substr": bson.A{"$contracts", 0, 1}},
				}}},
				{"amount", "$amount"},
				{"direction", "$side"},
				{"label",
					bson.D{
						{"$cond",
							bson.A{
								bson.D{{"$eq", bson.A{"$taker.orderId", objectID}}},
								bson.D{
									{"$arrayElemAt",
										bson.A{
											"$takerOrder.label",
											0,
										},
									},
								},
								bson.D{
									{"$arrayElemAt",
										bson.A{
											"$makerOrder.label",
											0,
										},
									},
								},
							},
						},
					},
				},
				{"order_id",
					bson.D{
						{"$cond",
							bson.A{
								bson.D{{"$eq", bson.A{"$taker.orderId", objectID}}},
								"$taker.orderId",
								"$maker.orderId",
							},
						},
					},
				},
				{"order_type",
					bson.D{
						{"$cond",
							bson.A{
								bson.D{{"$eq", bson.A{"$taker.orderId", objectID}}},
								bson.D{
									{"$arrayElemAt",
										bson.A{
											"$takerOrder.type",
											0,
										},
									},
								},
								bson.D{
									{"$arrayElemAt",
										bson.A{
											"$makerOrder.type",
											0,
										},
									},
								},
							},
						},
					},
				},
				{"price", "$price"},
				{"state",
					bson.D{
						{"$cond",
							bson.A{
								bson.D{{"$eq", bson.A{"$taker.orderId", objectID}}},
								bson.D{
									{"$arrayElemAt",
										bson.A{
											"$takerOrderId.status",
											0,
										},
									},
								},
								bson.D{
									{"$arrayElemAt",
										bson.A{
											"$makerOrder.status",
											0,
										},
									},
								},
							},
						},
					},
				},
				{"timestamp", bson.M{"$toLong": "$createdAt"}},
				{"strikePrice", "$strikePrice"},
				{"tickDirection", "$tickDirection"},
				{"tradeSequence", "$tradeSequence"},
				{"indexPrice", "$indexPrice"},
			},
		},
	}

	groupStage := bson.D{
		{"$group", bson.D{
			{"_id", nil},
			{"trades", bson.D{{"$push", "$$ROOT"}}},
		}},
	}

	pipeline := mongo.Pipeline{lookupMakerOrderStage, lookupTakerOrderStage, matchStage, projectStage, groupStage}

	var cursor *mongo.Cursor
	cursor, err = r.collection.Aggregate(context.Background(), pipeline, &options)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cursor.Close(context.Background())

	var res _tradeType.UserTradesByOderResult
	for cursor.Next(context.TODO()) {
		if err = cursor.Decode(&res); err != nil {
			fmt.Println(err)
			return
		}
		for _, trade := range res.Trades {
			trade.Api = true
		}
	}

	result.Trades = res.Trades

	return result, nil
}
