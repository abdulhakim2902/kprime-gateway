package repositories

import (
	"context"
	"fmt"
	deribitModel "gateway/internal/deribit/model"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	_types "gateway/internal/orderbook/types"

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

func (r OrderRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*_types.Order, error) {
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

	orders := []*_types.Order{}

	err = cursor.All(context.Background(), &orders)
	if err != nil {
		return nil, err
	}

	return orders, nil
}

func (r OrderRepository) GetInstruments(currency string, expired bool) ([]*deribitModel.DeribitGetInstrumentsResponse, error) {
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
		return []*deribitModel.DeribitGetInstrumentsResponse{}, nil
	}

	err = cursor.Err()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return []*deribitModel.DeribitGetInstrumentsResponse{}, err
	}

	orders := []*deribitModel.DeribitGetInstrumentsResponse{}

	if err = cursor.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)

		return []*deribitModel.DeribitGetInstrumentsResponse{}, nil
	}

	return orders, nil
}

func (r OrderRepository) GetOpenOrdersByInstrument(InstrumentName string, OrderType string, userId string) ([]*deribitModel.DeribitGetOpenOrdersByInstrumentResponse, error) {
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
		}}

	query := bson.M{
		"$match": bson.M{
			"orderState": bson.M{"$in": []_types.OrderStatus{_types.OPEN, _types.PARTIAL_FILLED}},
			"userId":     userId,
		},
	}

	sortStage := bson.M{
		"$sort": bson.M{
			"createdAt": -1,
		},
	}

	pipelineInstruments := bson.A{}

	pipelineInstruments = append(pipelineInstruments, projectStage)
	pipelineInstruments = append(pipelineInstruments, query)
	if OrderType != "all" {
		queryType := bson.M{
			"$match": bson.M{
				"orderType": bson.M{"$in": []_types.Type{_types.Type(strings.ToUpper(OrderType))}},
			},
		}
		pipelineInstruments = append(pipelineInstruments, queryType)
	}
	pipelineInstruments = append(pipelineInstruments, sortStage)

	cursor, err := r.collection.Aggregate(context.Background(), pipelineInstruments)
	if err != nil {
		fmt.Printf("err:%+v\n", err)

		return []*deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}, nil
	}

	err = cursor.Err()
	if err != nil {
		fmt.Printf("%+v\n", err)

		return []*deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}, err
	}

	orders := []*deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}

	if err = cursor.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)

		return []*deribitModel.DeribitGetOpenOrdersByInstrumentResponse{}, nil
	}

	return orders, nil
}

func (r OrderRepository) GetOrderHistoryByInstrument(InstrumentName string, Count int, Offset int, IncludeOld bool, IncludeUnfilled bool, userId string) ([]*deribitModel.DeribitGetOrderHistoryByInstrumentResponse, error) {
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
			"creationTimestamp": bson.M{"$toLong": "$createdAt"},
			"filledAmount":      "$filledAmount",
			"amount":            "$amount",
			"direction":         "$side",
			"usd":               "$price",
			"price":             "$price",
			"orderId":           "$_id",
			"timeInForce":       "$timeInForce",
			"orderType":         "$type",
			"orderState":        "$status",
			"userId":            "$userId",
		}}

	orderState := []_types.OrderStatus{_types.FILLED, _types.PARTIAL_FILLED}
	if IncludeUnfilled {
		orderState = append(orderState, _types.CANCELLED, _types.REJECTED)
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

		return []*deribitModel.DeribitGetOrderHistoryByInstrumentResponse{}, nil
	}

	err = cursor.Err()
	if err != nil {
		fmt.Printf("%+v\n", err)

		return []*deribitModel.DeribitGetOrderHistoryByInstrumentResponse{}, err
	}

	orders := []*deribitModel.DeribitGetOrderHistoryByInstrumentResponse{}

	if err = cursor.All(context.TODO(), &orders); err != nil {
		fmt.Printf("%+v\n", err)

		return []*deribitModel.DeribitGetOrderHistoryByInstrumentResponse{}, nil
	}

	return orders, nil
}

func (r OrderRepository) WsAggregate(pipeline interface{}) []*_types.WsOrder {
	opt := options.AggregateOptions{
		MaxTime: &defaultTimeout,
	}

	cursor, err := r.collection.Aggregate(context.Background(), pipeline, &opt)
	if err != nil {
		return []*_types.WsOrder{}
	}

	err = cursor.Err()
	if err != nil {
		return []*_types.WsOrder{}
	}

	orders := []*_types.WsOrder{}

	cursor.All(context.Background(), &orders)

	//sort orders by price
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].Price < orders[j].Price
	})

	return orders
}
