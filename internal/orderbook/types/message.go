package types

type Message struct {
	Instrument string      `json:"instrumentName"`
	Bids       interface{} `json:"bids"`
	Asks       interface{} `json:"asks"`
}
