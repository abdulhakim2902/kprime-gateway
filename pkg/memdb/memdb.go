package memdb

import (
	"gateway/schema"

	"github.com/hashicorp/go-memdb"
)

type MemDB struct {
	Name string
	Db   *memdb.MemDB
}

type Schemas struct {
	User *MemDB
}

func InitSchemas() (*Schemas, error) {
	user, err := InitSchema("user", schema.UserSchema)
	if err != nil {
		return nil, err
	}

	return &Schemas{user}, nil
}

func InitSchema(name string, schema *memdb.DBSchema) (*MemDB, error) {
	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}

	return &MemDB{Db: db, Name: name}, nil
}

func (m *MemDB) Find(index string, args ...interface{}) []interface{} {
	txn := m.Db.Txn(false)

	it, err := txn.Get(m.Name, index, args...)
	if err != nil {
		return []interface{}{}
	}

	res := []interface{}{}
	for obj := it.Next(); obj != nil; obj = it.Next() {
		res = append(res, obj)
	}

	return res
}

func (m *MemDB) FindOne(index string, args ...interface{}) (interface{}, error) {
	txn := m.Db.Txn(false)

	raw, err := txn.First(m.Name, index, args...)
	if err != nil {
		return nil, err
	}

	return raw, nil
}

func (m *MemDB) Create(data interface{}) error {
	txn := m.Db.Txn(true)

	if e := txn.Insert(m.Name, data); e != nil {
		return e
	}

	txn.Commit()

	return nil
}

func (m *MemDB) Update(data interface{}) error {
	txn := m.Db.Txn(true)

	if e := txn.Insert(m.Name, data); e != nil {
		return e
	}

	txn.Commit()

	return nil
}

func (m *MemDB) Delete(index string, obj interface{}) (interface{}, error) {
	txn := m.Db.Txn(true)

	if err := txn.Delete(m.Name, obj); err != nil {
		txn.Abort()

		return nil, err
	}

	txn.Commit()

	return nil, nil
}

func (m *MemDB) Clear(index string, args ...interface{}) (int, error) {
	txn := m.Db.Txn(true)

	status, err := txn.DeleteAll(m.Name, index, args...)
	if err != nil {
		txn.Abort()

		return status, err
	}

	txn.Commit()

	return status, nil
}
