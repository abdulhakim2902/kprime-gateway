package schema

import (
	"github.com/Undercurrent-Technologies/kprime-utilities/models/order"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"github.com/hashicorp/go-memdb"
)

type User struct {
	ID              string                 `json:"id"`
	OrderExclusions []order.OrderExclusion `json:"order_exclusions"`
	TypeInclusions  []order.TypeInclusions `json:"type_inclustion"`
	Role            types.UserRole         `json:"role"`
}

type UserCredential struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

var UserSchema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		"users": {
			Name: "users",
			Indexes: map[string]*memdb.IndexSchema{
				"id": {
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "ID"},
				},
				"order_exclusions": {
					Name:    "order_exclusions",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "OrderExclusions"},
				},
				"type_inclustion": {
					Name:    "type_inclustion",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "TypeInclusions"},
				},
				"role": {
					Name:    "role",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "Role"},
				},
			},
		},
	},
}

var UserCredentialSchema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		"user_credentials": {
			Name: "user_credentials",
			Indexes: map[string]*memdb.IndexSchema{
				"id": {
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "ID"},
				},
				"user_id": {
					Name:    "user_id",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "UserID"},
				},
				"key": {
					Name:    "key",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "Key"},
				},
				"secret": {
					Name:    "secret",
					Unique:  false,
					Indexer: &memdb.StringFieldIndex{Field: "Secret"},
				},
			},
		},
	},
}
