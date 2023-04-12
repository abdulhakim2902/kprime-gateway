package types

type Message struct {
	Instrument string      `json:"instrument_name"`
	Bids       interface{} `json:"bids"`
	Asks       interface{} `json:"asks"`
}

type ErrorMessage struct {
	Error string `json:"error"`
}
