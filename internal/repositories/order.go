package repositories

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	_deribitModel "gateway/internal/deribit/model"
	_orderbookType "gateway/internal/orderbook/types"
	"gateway/pkg/memdb"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type OrderRepository struct {
	collection *mongo.Collection
}

func NewOrderRepository(db Database) *OrderRepository {
	collection := db.InitCollection("orders")
	return &OrderRepository{collection}
}

var defaultTimeout = 10 * time.Second

func (r OrderRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*_orderbookType.Order, error) {
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

	orders := []*_orderbookType.Order{}

	err = cursor.All(context.Background(), &orders)
	if err != nil {
		return nil, err
	}

	return orders, nil
}

func (r OrderRepository) GetAvailableInstruments(currency string) ([]_deribitModel.DeribitResponse, error) {
	cur, err := r.collection.Find(context.Background(), bson.M{
		"underlying": currency,
	})
	if err != nil {
		return []_deribitModel.DeribitResponse{}, err
	}
	err = cur.Err()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return []_deribitModel.DeribitResponse{}, err
	}

	orders := []_deribitModel.DeribitResponse{}

	if err = cur.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)

		return []_deribitModel.DeribitResponse{}, nil
	}

	return orders, nil
}

func (r OrderRepository) GetInstruments(userId, currency string, expired bool) ([]*_deribitModel.DeribitGetInstrumentsResponse, error) {
	user, reason, err := memdb.MDBFindUserById(userId)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return []*_deribitModel.DeribitGetInstrumentsResponse{}, errors.New(reason.String())
	}

	now := time.Now()
	loc, _ := time.LoadLocation("Singapore")
	if loc != nil {
		now = now.In(loc)
	}

	projectStage := bson.M{
		"$project": bson.M{
			"InstrumentName": bson.M{"$concat": bson.A{
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
			}},
			"PriceIndex": bson.M{"$toLower": bson.M{"$concat": bson.A{
				bson.D{
					{"$convert", bson.D{
						{"input", "$underlying"},
						{"to", "string"},
					}}},
				"_USD",
			}}},
			"IsActive": bson.M{
				"$cond": bson.M{"if": bson.M{"$gt": []interface{}{bson.M{"$toDate": "$expiryDate"}, now}},
					"then": true,
					"else": false}},
			"BaseCurrency": bson.M{"$concat": bson.A{
				bson.D{
					{"$convert", bson.D{
						{"input", "$underlying"},
						{"to", "string"},
					}}},
			}},
			"ContractSize": bson.D{
				{"$convert", bson.D{
					{"input", 1},
					{"to", "int"},
				}}},
			"ExpirationTimestamp": bson.M{"$toLong": bson.M{"$toDate": "$expiryDate"}},
			"CreationTimestamp":   bson.M{"$toLong": "$createdAt"},
			"Kind": bson.M{"$concat": bson.A{
				bson.D{
					{"$convert", bson.D{
						{"input", "option"},
						{"to", "string"},
					}}},
			}},
			"QuoteCurrency":      "USD",
			"SettlementCurrency": "USD",
			"Strike":             "$strikePrice",
			"OptionType":         bson.M{"$toLower": "$contracts"},
			"underlying":         "$underlying",
			"userId":             "$userId",
			"userRole":           "$userRole",
		}}

	match := bson.M{
		"underlying": currency,
		"IsActive":   !expired,
	}

	if user.Role == types.CLIENT {
		excludeUserId := []string{}

		for _, userCast := range user.OrderExclusions {
			excludeUserId = append(excludeUserId, userCast.UserID)
		}

		match["userRole"] = types.MARKET_MAKER.String()
		match["userId"] = bson.M{"$nin": excludeUserId}
	}

	matchesStage := bson.M{"$match": match}

	groupStage := bson.M{
		"$group": bson.M{
			"_id": bson.M{
				"InstrumentName": "$InstrumentName",
			},
			"InstrumentName": bson.M{
				"$first": "$InstrumentName",
			},
			"PriceIndex": bson.M{
				"$first": "$PriceIndex",
			},
			"IsActive": bson.M{
				"$first": "$IsActive",
			},
			"BaseCurrency": bson.M{
				"$first": "$BaseCurrency",
			},
			"ContractSize": bson.M{
				"$first": "$ContractSize",
			},
			"CreationTimestamp": bson.M{
				"$first": "$CreationTimestamp",
			},
			"ExpirationTimestamp": bson.M{
				"$first": "$ExpirationTimestamp",
			},
			"Kind": bson.M{
				"$first": "$Kind",
			},
			"QuoteCurrency": bson.M{
				"$first": "$QuoteCurrency",
			},
			"SettlementCurrency": bson.M{
				"$first": "$SettlementCurrency",
			},
			"Strike": bson.M{
				"$first": "$Strike",
			},
			"OptionType": bson.M{
				"$first": "$OptionType",
			},
		},
	}
	sortStage := bson.M{
		"$sort": bson.M{
			"CreationTimestamp": -1,
		},
	}

	pipelineInstruments := bson.A{}

	pipelineInstruments = append(pipelineInstruments, projectStage)
	pipelineInstruments = append(pipelineInstruments, matchesStage)
	pipelineInstruments = append(pipelineInstruments, groupStage)
	pipelineInstruments = append(pipelineInstruments, sortStage)

	cursor, err := r.collection.Aggregate(context.Background(), pipelineInstruments)
	if err != nil {
		return []*_deribitModel.DeribitGetInstrumentsResponse{}, nil
	}

	err = cursor.Err()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return []*_deribitModel.DeribitGetInstrumentsResponse{}, err
	}

	orders := []*_deribitModel.DeribitGetInstrumentsResponse{}

	if err = cursor.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)

		return []*_deribitModel.DeribitGetInstrumentsResponse{}, nil
	}

	return orders, nil
}

func (r OrderRepository) GetOpenOrdersByInstrument(InstrumentName string, OrderType string, userId string) ([]*_deribitModel.DeribitGetOpenOrdersByInstrumentResponse, error) {
	projectStage := bson.M{
		"$project": bson.M{
			"InstrumentName": bson.M{"$concat": bson.A{
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
			}},
			"replaced": bson.M{
				"$cond": bson.M{"if": bson.M{"$and": []interface{}{bson.M{"$eq": []interface{}{bson.M{"$type": "$amendments"}, "array"}}, bson.M{"$ne": []interface{}{"$amendments", "[]"}}}},
					"then": true,
					"else": false}},
			"filledAmount": bson.D{
				{"$convert", bson.D{
					{"input", "$filledAmount"},
					{"to", "double"},
				}},
			},
			"amount":      "$amount",
			"direction":   "$side",
			"price":       "$price",
			"orderId":     "$_id",
			"timeInForce": "$timeInForce",
			"orderType":   "$type",
			"orderState":  "$status",
			"userId":      "$userId",

			"label":               "$label",
			"usd":                 "$price",
			"api":                 bson.M{"$toBool": "true"},
			"creationTimestamp":   bson.M{"$toLong": "$createdAt"},
			"lastUpdateTimestamp": bson.M{"$toLong": "$updatedAt"},
			"cancelledReason":     canceledReasonQuery(),
			"priceAvg": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$gt", bson.A{"$tradePriceAvg.price", 0}}}},
					{"then", "$tradePriceAvg.price"},
					{"else", primitive.Null{}},
				},
			},
		},
	}

	query := bson.M{
		"$match": bson.M{
			"orderState": bson.M{"$in": []types.OrderStatus{types.OPEN, types.PARTIAL_FILLED}},
			"userId":     userId,
		},
	}

	sortStage := bson.M{
		"$sort": bson.M{
			"createdAt": -1,
		},
	}

	pipelineInstruments := bson.A{}

	priceAvgStage, err := tradePriceAvgQuery(InstrumentName)
	if err != nil {
		return nil, err
	}
	pipelineInstruments = append(pipelineInstruments, priceAvgStage...)

	pipelineInstruments = append(pipelineInstruments, projectStage)
	pipelineInstruments = append(pipelineInstruments, query)
	if OrderType != "all" {
		queryType := bson.M{
			"$match": bson.M{
				"orderType": bson.M{"$in": []types.Type{types.Type(strings.ToUpper(OrderType))}},
			},
		}
		pipelineInstruments = append(pipelineInstruments, queryType)
	}
	pipelineInstruments = append(pipelineInstruments, sortStage)

	cursor, err := r.collection.Aggregate(context.Background(), pipelineInstruments)
	if err != nil {
		fmt.Printf("err:%+v\n", err)

		return []*_deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}, nil
	}

	err = cursor.Err()
	if err != nil {
		fmt.Printf("%+v\n", err)

		return []*_deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}, err
	}

	orders := []*_deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}

	if err = cursor.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)

		return []*_deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}, nil
	}

	return orders, nil
}

func (r OrderRepository) GetMarketData(instrumentName string, side string) (res []_deribitModel.DeribitResponse) {
	splits := strings.Split(instrumentName, "-")
	fmt.Println(splits, side)
	price, _ := strconv.ParseFloat(splits[2], 64)
	fmt.Println(price)
	curr, err := r.collection.Find(context.Background(), bson.M{
		"underlying":  splits[0],
		"expiryDate":  splits[1],
		"strikePrice": price,
		"side":        side,
	})
	if err != nil {
		fmt.Printf("%+v\n", err)
	}

	if err = curr.All(context.TODO(), &res); err != nil {
		fmt.Printf("%+v\n", err)
	}

	return res
}

func (r OrderRepository) GetOrderHistoryByInstrument(InstrumentName string, Count int, Offset int, IncludeOld bool, IncludeUnfilled bool, userId string) ([]*_deribitModel.DeribitGetOrderHistoryByInstrumentResponse, error) {
	now := time.Now()
	loc, _ := time.LoadLocation("Singapore")
	if loc != nil {
		now = now.In(loc)
	}
	now = now.Add(2 * -24 * time.Hour)

	projectStage := bson.M{
		"$project": bson.M{
			"InstrumentName": bson.M{"$concat": bson.A{
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
			}},
			"replaced": bson.M{
				"$cond": bson.M{"if": bson.M{"$and": []interface{}{bson.M{"$eq": []interface{}{bson.M{"$type": "$amendments"}, "array"}}, bson.M{"$ne": []interface{}{"$amendments", "[]"}}}},
					"then": true,
					"else": false}},
			"filledAmount": bson.D{
				{"$convert", bson.D{
					{"input", "$filledAmount"},
					{"to", "double"},
				}},
			},
			"amount":      "$amount",
			"direction":   "$side",
			"price":       "$price",
			"orderId":     "$_id",
			"timeInForce": "$timeInForce",
			"orderType":   "$type",
			"orderState":  "$status",
			"userId":      "$userId",

			"label":               "$label",
			"usd":                 "$price",
			"api":                 bson.M{"$toBool": "true"},
			"creationTimestamp":   bson.M{"$toLong": "$createdAt"},
			"lastUpdateTimestamp": bson.M{"$toLong": "$updatedAt"},
			"cancelledReason":     canceledReasonQuery(),
			"priceAvg": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$gt", bson.A{"$tradePriceAvg.price", 0}}}},
					{"then", "$tradePriceAvg.price"},
					{"else", primitive.Null{}},
				},
			},
		}}

	orderState := []types.OrderStatus{types.FILLED, types.PARTIAL_FILLED}
	if IncludeUnfilled {
		orderState = append(orderState, types.CANCELLED)
	}
	query := bson.M{
		"$match": bson.M{
			"orderState": bson.M{"$in": orderState},
			"userId":     userId,
		},
	}

	limitStage := bson.M{
		"$limit": Count,
	}

	skipStage := bson.M{
		"$skip": Offset,
	}

	sortStage := bson.M{
		"$sort": bson.M{
			"creationTimestamp": -1,
		},
	}

	pipelineInstruments := bson.A{}

	priceAvgStage, err := tradePriceAvgQuery(InstrumentName)
	if err != nil {
		return nil, err
	}
	pipelineInstruments = append(pipelineInstruments, priceAvgStage...)

	pipelineInstruments = append(pipelineInstruments, projectStage)
	pipelineInstruments = append(pipelineInstruments, query)
	if !IncludeOld {
		queryTimestamp := bson.M{
			"$match": bson.M{
				"creationTimestamp": bson.M{"$gte": now.UnixMilli()},
			},
		}
		pipelineInstruments = append(pipelineInstruments, queryTimestamp)
	}
	pipelineInstruments = append(pipelineInstruments, skipStage)
	pipelineInstruments = append(pipelineInstruments, limitStage)
	pipelineInstruments = append(pipelineInstruments, sortStage)

	cursor, err := r.collection.Aggregate(context.Background(), pipelineInstruments)
	if err != nil {
		fmt.Printf("err:%+v\n", err)

		return []*_deribitModel.DeribitGetOrderHistoryByInstrumentResponse{}, nil
	}

	err = cursor.Err()
	if err != nil {
		fmt.Printf("%+v\n", err)

		return []*_deribitModel.DeribitGetOrderHistoryByInstrumentResponse{}, err
	}

	orders := []*_deribitModel.DeribitGetOrderHistoryByInstrumentResponse{}

	if err = cursor.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)

		return []*_deribitModel.DeribitGetOrderHistoryByInstrumentResponse{}, nil
	}

	return orders, nil
}

func (r OrderRepository) GetChangeOrdersByInstrument(InstrumentName string, userId []interface{}, orderId []interface{}) ([]_deribitModel.DeribitGetOpenOrdersByInstrumentResponse, error) {
	projectStage := bson.M{
		"$project": bson.M{
			"InstrumentName": bson.M{"$concat": bson.A{
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
			}},
			"replaced": bson.M{
				"$cond": bson.M{"if": bson.M{"$and": []interface{}{bson.M{"$eq": []interface{}{bson.M{"$type": "$amendments"}, "array"}}, bson.M{"$ne": []interface{}{"$amendments", "[]"}}}},
					"then": true,
					"else": false}},
			"filledAmount": bson.D{
				{"$convert", bson.D{
					{"input", "$filledAmount"},
					{"to", "double"},
				}},
			},
			"amount":              "$amount",
			"direction":           "$side",
			"price":               "$price",
			"orderId":             "$_id",
			"timeInForce":         "$timeInForce",
			"orderType":           "$type",
			"orderState":          "$status",
			"userId":              "$userId",
			"label":               "$label",
			"usd":                 "$price",
			"api":                 bson.M{"$toBool": "true"},
			"creationTimestamp":   bson.M{"$toLong": "$createdAt"},
			"lastUpdateTimestamp": bson.M{"$toLong": "$updatedAt"},
			"cancelledReason":     canceledReasonQuery(),
			"maxShow":             "$maxShow",
			"priceAvg": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$gt", bson.A{"$tradePriceAvg.price", 0}}}},
					{"then", "$tradePriceAvg.price"},
					{"else", primitive.Null{}},
				},
			},
		},
	}

	query := bson.M{
		"$match": bson.M{
			"_id":    bson.M{"$in": orderId},
			"userId": bson.M{"$in": userId},
		},
	}

	sortStage := bson.M{
		"$sort": bson.M{
			"createdAt": -1,
		},
	}

	pipelineInstruments := bson.A{}

	priceAvgStage, err := tradePriceAvgQuery(InstrumentName)
	if err != nil {
		return nil, err
	}
	pipelineInstruments = append(pipelineInstruments, priceAvgStage...)

	pipelineInstruments = append(pipelineInstruments, projectStage)
	pipelineInstruments = append(pipelineInstruments, query)
	pipelineInstruments = append(pipelineInstruments, sortStage)

	cursor, err := r.collection.Aggregate(context.Background(), pipelineInstruments)
	if err != nil {
		fmt.Printf("err:%+v\n", err)

		return []_deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}, nil
	}

	err = cursor.Err()
	if err != nil {
		fmt.Printf("%+v\n", err)

		return []_deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}, err
	}

	orders := []_deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}

	if err = cursor.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)

		return []_deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}, nil
	}
	return orders, nil
}

func (r OrderRepository) WsAggregate(pipeline interface{}) []*_orderbookType.WsOrder {
	opt := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	cursor, err := r.collection.Aggregate(context.Background(), pipeline, &opt)
	if err != nil {
		return []*_orderbookType.WsOrder{}
	}

	err = cursor.Err()
	if err != nil {
		return []*_orderbookType.WsOrder{}
	}

	orders := []*_orderbookType.WsOrder{}

	cursor.All(context.Background(), &orders)

	//sort orders by price
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].Price < orders[j].Price
	})

	return orders
}

func (r OrderRepository) CountAggregate(pipeline interface{}) []_orderbookType.Count {
	opt := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	cursor, err := r.collection.Aggregate(context.Background(), pipeline, &opt)
	if err != nil {
		return []_orderbookType.Count{}
	}

	err = cursor.Err()
	if err != nil {
		return []_orderbookType.Count{}
	}

	counts := []_orderbookType.Count{}

	cursor.All(context.Background(), &counts)

	return counts
}

func canceledReasonQuery() bson.D {
	return bson.D{
		{"$switch",
			bson.D{
				{"branches",
					bson.A{
						bson.D{{"case", bson.D{{"$eq", bson.A{"$cancelledReason", 1}}}}, {"then", "user_request"}},
						bson.D{{"case", bson.D{{"$eq", bson.A{"$cancelledReason", 2}}}}, {"then", "immediate_or_cancel"}},
						bson.D{{"case", bson.D{{"$eq", bson.A{"$cancelledReason", 3}}}}, {"then", "good_til_day"}},
					},
				},
				{"default", "none"},
			},
		},
	}
}

func tradePriceAvgQuery(instrument string) (query bson.A, err error) {
	substring := strings.Split(instrument, "-")
	if len(substring) != 4 {
		err = fmt.Errorf("invalid instrument name")
		return
	}

	var strikePrice float64
	strikePrice, err = strconv.ParseFloat(substring[2], 64)
	if err != nil {
		return
	}
	_contracts := ""
	if substring[3] == "P" {
		_contracts = "PUT"
	} else {
		_contracts = "CALL"
	}

	query = bson.A{
		bson.M{
			"$lookup": bson.D{
				{"from", "trades"},
				{"pipeline",
					bson.A{
						bson.D{
							{"$match",
								bson.D{
									{"underlying", substring[0]},
									{"expiryDate", substring[1]},
									{"strikePrice", strikePrice},
									{"contracts", _contracts},
								},
							},
						},
						bson.D{
							{"$group",
								bson.D{{"_id", primitive.Null{}}, {"price", bson.D{{"$avg", "$price"}}}},
							},
						},
						bson.D{{"$unset", bson.A{"_id"}}},
					},
				},
				{"as", "tradePriceAvg"},
			},
		},
		bson.M{"$unwind": bson.M{
			"path":                       "$tradePriceAvg",
			"preserveNullAndEmptyArrays": true,
		}},
	}

	return
}

func (r OrderRepository) GetOrderBook(o _orderbookType.GetOrderBook) *_orderbookType.Orderbook {

	queryBuilder := func(side types.Side, priceOrder int) interface{} {
		match := bson.M{
			"status":      bson.M{"$in": []types.OrderStatus{types.OPEN, types.PARTIAL_FILLED}},
			"underlying":  o.Underlying,
			"strikePrice": o.StrikePrice,
			"expiryDate":  o.ExpiryDate,
			"side":        side,
		}

		if o.UserRole == types.CLIENT.String() {
			match["userRole"] = types.MARKET_MAKER.String()
			match["userId"] = bson.M{"$nin": o.UserOrderExclusions}
		}

		return []bson.M{
			{
				"$match": match,
			},
			{
				"$group": bson.D{
					{"_id", "$price"},
					{"amount", bson.D{{"$sum", bson.M{"$subtract": bson.A{
						"$amount",
						bson.M{"$toDouble": "$filledAmount"},
					}}}}},
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
	asks := r.WsAggregate(asksQuery)

	bidsQuery := queryBuilder(types.BUY, 1)
	bids := r.WsAggregate(bidsQuery)

	orderbooks := &_orderbookType.Orderbook{
		InstrumentName: o.InstrumentName,
		Asks:           asks,
		Bids:           bids,
	}

	return orderbooks
}

func (r OrderRepository) GetOrderBookAgg2(o _orderbookType.GetOrderBook) _orderbookType.Orderbook {
	queryBuilderCount := func(side types.Side) interface{} {
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
			{
				"$count": "count",
			},
		}
	}
	queryBuilder := func(side types.Side, priceOrder int, buckets int) interface{} {
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
			{
				"$bucketAuto": bson.D{
					{"groupBy", "$price"},
					{"buckets", buckets},
					{"output", bson.M{
						"amount": bson.D{{"$sum", "$amount"}},
						"price":  bson.D{{"$min", "$price"}},
					},
					},
				},
			},
		}
	}

	countAsk := queryBuilderCount(types.SELL)
	countA := r.CountAggregate(countAsk)

	countBid := queryBuilderCount(types.BUY)
	countB := r.CountAggregate(countBid)

	var asks []*_orderbookType.WsOrder
	if len(countA) > 0 {
		buckets := (countA[0].Count + 1) / 2
		asksQuery := queryBuilder(types.SELL, -1, buckets)
		asks = r.WsAggregate(asksQuery)
	}

	var bids []*_orderbookType.WsOrder
	if len(countB) > 0 {
		buckets := (countB[0].Count + 1) / 2
		bidsQuery := queryBuilder(types.BUY, 1, buckets)
		bids = r.WsAggregate(bidsQuery)
	}

	orderbooks := _orderbookType.Orderbook{
		InstrumentName: o.InstrumentName,
		Asks:           asks,
		Bids:           bids,
	}

	return orderbooks
}

func (r OrderRepository) GetOrderLatestTimestamp(o _orderbookType.GetOrderBook, after int64, isFilled bool) _orderbookType.Orderbook {
	timeAfter := time.UnixMilli(after)
	status := []types.OrderStatus{types.OPEN, types.PARTIAL_FILLED}
	if isFilled {
		status = append(status, types.FILLED)
	}
	queryBuilder := func(side types.Side, priceOrder int) interface{} {
		return []bson.M{
			{
				"$match": bson.M{
					"status":      bson.M{"$in": status},
					"underlying":  o.Underlying,
					"strikePrice": o.StrikePrice,
					"expiryDate":  o.ExpiryDate,
					"side":        side,
					"updatedAt":   bson.M{"$lt": timeAfter},
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
	asks := r.WsAggregate(asksQuery)

	bidsQuery := queryBuilder(types.BUY, 1)
	bids := r.WsAggregate(bidsQuery)

	orderbooks := _orderbookType.Orderbook{
		InstrumentName: o.InstrumentName,
		Asks:           asks,
		Bids:           bids,
	}

	return orderbooks
}

func (r OrderRepository) GetOrderLatestTimestampAgg(o _orderbookType.GetOrderBook, after int64) _orderbookType.Orderbook {
	timeAfter := time.UnixMilli(after)
	queryBuilderCount := func(side types.Side) interface{} {
		return []bson.M{
			{
				"$match": bson.M{
					"status":      bson.M{"$in": []types.OrderStatus{types.OPEN, types.PARTIAL_FILLED}},
					"underlying":  o.Underlying,
					"strikePrice": o.StrikePrice,
					"expiryDate":  o.ExpiryDate,
					"side":        side,
					"updatedAt":   bson.M{"$lt": timeAfter},
				},
			},
			{
				"$group": bson.D{
					{"_id", "$price"},
					{"amount", bson.D{{"$sum", bson.M{"$subtract": []string{"$amount", "$filledAmount"}}}}},
					{"detail", bson.D{{"$first", "$$ROOT"}}},
				},
			},
			{
				"$count": "count",
			},
		}
	}
	queryBuilder := func(side types.Side, priceOrder int, buckets int) interface{} {
		return []bson.M{
			{
				"$match": bson.M{
					"status":      bson.M{"$in": []types.OrderStatus{types.OPEN, types.PARTIAL_FILLED}},
					"underlying":  o.Underlying,
					"strikePrice": o.StrikePrice,
					"expiryDate":  o.ExpiryDate,
					"side":        side,
					"updatedAt":   bson.M{"$lt": timeAfter},
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
			{
				"$bucketAuto": bson.D{
					{"groupBy", "$price"},
					{"buckets", buckets},
					{"output", bson.M{
						"amount": bson.D{{"$sum", "$amount"}},
						"price":  bson.D{{"$min", "$price"}},
					},
					},
				},
			},
		}
	}

	countAsk := queryBuilderCount(types.SELL)
	countA := r.CountAggregate(countAsk)

	countBid := queryBuilderCount(types.BUY)
	countB := r.CountAggregate(countBid)
	var asks []*_orderbookType.WsOrder
	if len(countA) > 0 {
		buckets := (countA[0].Count + 1) / 2
		asksQuery := queryBuilder(types.SELL, -1, buckets)
		asks = r.WsAggregate(asksQuery)
	}

	var bids []*_orderbookType.WsOrder
	if len(countB) > 0 {
		buckets := (countB[0].Count + 1) / 2
		bidsQuery := queryBuilder(types.BUY, 1, buckets)
		bids = r.WsAggregate(bidsQuery)
	}

	orderbooks := _orderbookType.Orderbook{
		InstrumentName: o.InstrumentName,
		Asks:           asks,
		Bids:           bids,
	}

	return orderbooks
}

func (r OrderRepository) GetOrderState(userId string, orderId string) ([]_deribitModel.DeribitGetOrderStateResponse, error) {
	projectStage := bson.M{
		"$project": bson.M{
			"InstrumentName": bson.M{"$concat": bson.A{
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
			}},
			"replaced": bson.M{
				"$cond": bson.M{"if": bson.M{"$and": []interface{}{bson.M{"$eq": []interface{}{bson.M{"$type": "$amendments"}, "array"}}, bson.M{"$ne": []interface{}{"$amendments", "[]"}}}},
					"then": true,
					"else": false}},
			"filledAmount": bson.D{
				{"$convert", bson.D{
					{"input", "$filledAmount"},
					{"to", "double"},
				}},
			},
			"amount":              "$amount",
			"direction":           "$side",
			"price":               "$price",
			"orderId":             "$_id",
			"timeInForce":         "$timeInForce",
			"orderType":           "$type",
			"orderState":          "$status",
			"userId":              "$userId",
			"label":               "$label",
			"usd":                 "$price",
			"api":                 bson.M{"$toBool": "true"},
			"creationTimestamp":   bson.M{"$toLong": "$createdAt"},
			"lastUpdateTimestamp": bson.M{"$toLong": "$updatedAt"},
			"cancelledReason":     canceledReasonQuery(),
			"priceAvg": bson.M{
				"$cond": bson.D{
					{"if", bson.D{{"$gt", bson.A{"$tradePriceAvg.price", 0}}}},
					{"then", "$tradePriceAvg.price"},
					{"else", primitive.Null{}},
				},
			},
		},
	}

	objectID, err := primitive.ObjectIDFromHex(orderId)
	if err != nil {
		return []_deribitModel.DeribitGetOrderStateResponse{}, err
	}
	pipelineInstruments := bson.A{}

	query := bson.M{
		"$match": bson.M{
			"_id":    objectID,
			"userId": userId,
		},
	}
	pipelineInstruments = append(pipelineInstruments, query)

	sortStage := bson.M{
		"$sort": bson.M{
			"createdAt": -1,
		},
	}

	pipelineInstruments = append(pipelineInstruments, projectStage)
	// pipelineInstruments = append(pipelineInstruments, query)
	pipelineInstruments = append(pipelineInstruments, sortStage)

	cursor, err := r.collection.Aggregate(context.Background(), pipelineInstruments)
	if err != nil {
		return []_deribitModel.DeribitGetOrderStateResponse{}, nil
	}

	orders := []_deribitModel.DeribitGetOrderStateResponse{}

	if err = cursor.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)
		return []_deribitModel.DeribitGetOrderStateResponse{}, nil
	}

	return orders, nil
}

func (r OrderRepository) GetOrderStateByLabel(ctx context.Context, req _deribitModel.DeribitGetOrderStateByLabelRequest) (orders []*_deribitModel.DeribitGetOrderStateByLabelResponse, err error) {

	projectStage := bson.M{
		"$project": bson.M{
			"InstrumentName": bson.M{"$concat": bson.A{
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
			}},
			"replaced": bson.M{
				"$cond": bson.M{"if": bson.M{"$and": []interface{}{bson.M{"$eq": []interface{}{bson.M{"$type": "$amendments"}, "array"}}, bson.M{"$ne": []interface{}{"$amendments", "[]"}}}},
					"then": true,
					"else": false}},
			"filledAmount": bson.D{
				{"$convert", bson.D{
					{"input", "$filledAmount"},
					{"to", "double"},
				}},
			},
			"amount":      "$amount",
			"direction":   "$side",
			"price":       "$price",
			"orderId":     "$_id",
			"timeInForce": "$timeInForce",
			"orderType":   "$type",
			"orderState":  "$status",

			"label":               "$label",
			"usd":                 "$price",
			"api":                 bson.M{"$toBool": "true"},
			"creationTimestamp":   bson.M{"$toLong": "$createdAt"},
			"lastUpdateTimestamp": bson.M{"$toLong": "$updatedAt"},
			"cancelledReason":     canceledReasonQuery(),
			// "priceAvg": bson.M{
			// 	"$cond": bson.D{
			// 		{"if", bson.D{{"$gt", bson.A{"$tradePriceAvg.price", 0}}}},
			// 		{"then", "$tradePriceAvg.price"},
			// 		{"else", primitive.Null{}},
			// 	},
			// },
		}}

	query := bson.M{
		"$match": bson.M{
			"underlying": req.Currency,
			"label":      req.Label,
			"userId":     req.UserId,
		},
	}

	pipelineInstruments := bson.A{}
	// priceAvgStage, err := tradePriceAvgQuery(InstrumentName)
	// if err != nil {
	// 	return nil, err
	// }
	// pipelineInstruments = append(pipelineInstruments, priceAvgStage...)

	pipelineInstruments = append(pipelineInstruments, query)
	pipelineInstruments = append(pipelineInstruments, projectStage)

	var cursor *mongo.Cursor
	cursor, err = r.collection.Aggregate(context.Background(), pipelineInstruments)
	if err != nil {
		fmt.Printf("err:%+v\n", err)

		return
	}

	if err = cursor.Err(); err != nil {
		fmt.Printf("%+v\n", err)

		return
	}

	if err = cursor.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)

		return
	}

	return
}
