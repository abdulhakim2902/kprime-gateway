package model

type Response struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Paging  interface{} `json:"paging"`
}
