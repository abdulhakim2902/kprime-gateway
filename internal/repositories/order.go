package repositories

import (
	"context"
	"fmt"
	deribitModel "gateway/internal/deribit/model"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"gateway/internal/repositories/types"

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

func (r OrderRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*types.Order, error) {
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

	orders := []*types.Order{}

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
