package types

type Side string
type Type string
type Contracts string
type OrderStatus string

const (
	BUY    Side = "BUY"
	SELL   Side = "SELL"
	EDIT   Side = "EDIT"
	CANCEL Side = "CANCEL"
)

const (
	LIMIT  Type = "LIMIT"
	MARKET Type = "MARKET"
)

const (
	PUT  Contracts = "PUT"
	CALL Contracts = "CALL"
)

const (
	OPEN           OrderStatus = "OPEN"
	PARTIAL_FILLED OrderStatus = "PARTIAL_FILLED"
	FILLED         OrderStatus = "FILLED"
	CANCELLED      OrderStatus = "CANCELLED"
)
