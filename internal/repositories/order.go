package repositories

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"git.devucc.name/dependencies/utilities/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	_deribitModel "gateway/internal/deribit/model"
	_orderbookType "gateway/internal/orderbook/types"

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

func (r OrderRepository) GetInstruments(currency string, expired bool) ([]*_deribitModel.DeribitGetInstrumentsResponse, error) {
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
			"PriceIndex": bson.M{"$concat": bson.A{
				bson.D{
					{"$convert", bson.D{
						{"input", "$underlying"},
						{"to", "string"},
					}}},
				"-USD",
			}},
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
			"QuoteCurrency": "USD",
			"underlying":    "$underlying",
		}}
	matchUnerlyingStage := bson.M{
		"$match": bson.M{
			"underlying": currency,
		},
	}

	matchIsActiveStage := bson.M{
		"$match": bson.M{
			"IsActive": !expired,
		},
	}
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
		},
	}
	sortStage := bson.M{
		"$sort": bson.M{
			"CreationTimestamp": -1,
		},
	}

	pipelineInstruments := bson.A{}

	pipelineInstruments = append(pipelineInstruments, projectStage)
	pipelineInstruments = append(pipelineInstruments, matchUnerlyingStage)
	pipelineInstruments = append(pipelineInstruments, matchIsActiveStage)
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
			"filledAmount": "$filledAmount",
			"amount":       "$amount",
			"direction":    "$side",
			"price":        "$price",
			"orderId":      "$_id",
			"timeInForce":  "$timeInForce",
			"orderType":    "$type",
			"orderState":   "$status",
			"userId":       "$userId",

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
			"filledAmount": "$filledAmount",
			"amount":       "$amount",
			"direction":    "$side",
			"price":        "$price",
			"orderId":      "$_id",
			"timeInForce":  "$timeInForce",
			"orderType":    "$type",
			"orderState":   "$status",
			"userId":       "$userId",

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
		orderState = append(orderState, types.CANCELLED, types.REJECTED)
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
