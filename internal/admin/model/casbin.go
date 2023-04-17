package model

type Casbin struct {
	Id    uint   `gorm:"primarykey" json:"id"`
	Ptype string `json:"ptype"`
	V0    string `json:"v0"` //role name
	V1    string `json:"v1"` //resource
	V2    string `json:"v2"` //operation: read/write/delete
	v3    string
	v4    string
	v5    string
}

type Tabler interface {
	TableName() string
}

// TableName overrides the table name used by User to `profiles`
func (Casbin) TableName() string {
	return "casbin_rule"
}
