package model

type Casbin struct {
	ptype string
	v0    string //role name
	v1    string //resource
	v2    string //operation: read/write/delete
	v3    string
	v4    string
	v5    string
}