package model

type Response struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	DataId  interface{} `json:"dataId"`
	Paging  interface{} `json:"paging"`
}
