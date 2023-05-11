package repositories

import (
	"context"
	"errors"
	"gateway/internal/auth/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AuthRepository struct {
	collection *mongo.Collection
}

func NewAuthRepositoryRepository(db Database) *AuthRepository {
	collection := db.InitCollection("users")
	return &AuthRepository{collection}
}

func (r AuthRepository) Find(filter interface{}, sort interface{}, offset, limit int64) ([]*types.User, error) {
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

	users := []*types.User{}

	err = cursor.All(context.Background(), &users)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (r AuthRepository) FindOne(filter interface{}) (user *types.User, err error) {
	res := r.collection.FindOne(context.Background(), filter)
	if err = res.Err(); err != nil {
		return
	}

	err = res.Decode(&user)

	return
}

func (r *AuthRepository) FindByAPIKeyAndSecret(ctx context.Context, apiKey, apiSecret string) (user *types.User, err error) {
	user, err = r.FindOne(bson.D{
		{"$and",
			bson.A{
				bson.D{{"apiCredentials.apiKey", bson.D{{"$eq", apiKey}}}},
				bson.D{{"apiCredentials.apiSecret", bson.D{{"$eq", apiSecret}}}},
			},
		},
	})

	return
}

func (r *AuthRepository) FindById(ctx context.Context, userId string) (user *types.User, err error) {
	var id primitive.ObjectID
	id, err = primitive.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errors.New("invalid id")
	}
	user, err = r.FindOne(bson.M{"_id": id})

	return
}
