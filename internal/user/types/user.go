package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID              primitive.ObjectID `json:"id" bson:"_id"`
	Email           string             `json:"email" bson:"email"`
	Password        string             `json:"password" bson:"password"`
	Role            Role               `json:"role" bson:"role"`
	OrderTypes      []*OrderType       `json:"orderTypes" bson:"orderTypes"`
	OrderExclusions []*OrderExclusions `json:"orderExclusions" bson:"orderExclusions"`
	APICredentials  []*APICredentials  `json:"apiCredentials" bson:"apiCredentials"`
	CreatedAt       time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt" bson:"updatedAt"`
}

type Role struct {
	ID        primitive.ObjectID `json:"id" bson:"_id"`
	Name      string             `json:"name" bson:"name"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

type OrderType struct {
	ID        primitive.ObjectID `json:"id" bson:"_id"`
	Name      string             `json:"name" bson:"name"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

type OrderExclusions struct {
	ID        primitive.ObjectID `json:"id" bson:"_id"`
	UserID    string             `json:"name" bson:"userId"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

type APICredentials struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	APIKey       string             `json:"apiKey" bson:"apiKey"`
	APISecret    string             `json:"apiSecret" bson:"apiSecret"`
	Permissions  []interface{}      `json:"permissions" bson:"permissions"`
	IPWhitelists []interface{}      `json:"ipWhitelists" bson:"ipWhitelists"`
	CreatedAt    time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time          `json:"updatedAt" bson:"updatedAt"`
}
