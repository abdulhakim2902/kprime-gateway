package protocol

import (
	"gateway/pkg/ws"
	"net/http"
	"strconv"
	"time"

	"git.devucc.name/dependencies/utilities/types/validation_reason"
	"github.com/gin-gonic/gin"
)

type ProtocolType int

const (
	Websocket ProtocolType = iota
	GET
	POST
	GRPC
)

type RPCResponseMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      uint64      `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	UsIn    uint64      `json:"usIn,omitempty"`
	UsOut   uint64      `json:"usOut,omitempty"`
	UsDiff  uint64      `json:"usDiff,omitempty"`
	Testnet bool        `json:"testnet,omitempty"`

	// Error Type
	Error ErrorMessage `json:"error,omitempty"`
}

type ErrorMessage struct {
	Message string        `json:"message"`
	Data    ReasonMessage `json:"data"`
	Code    int64         `json:"code"`
}

type ReasonMessage struct {
	Reason string `json:"reason"`
}

type ProtocolRequest struct {
	Protocol      ProtocolType
	RPCID         uint64
	UserID        string
	RequestedTime uint64
	ws            *ws.Client
	gin           *gin.Context
}

var protocolConnections map[string]ProtocolRequest

func checkKeyExists(key string) bool {
	_, ok := protocolConnections[key]
	return ok
}

func RegisterProtocolRequest(protocol ProtocolType, rpcID uint64, userID string, ws *ws.Client, gin *gin.Context) bool {
	if protocolConnections == nil {
		protocolConnections = make(map[string]ProtocolRequest)
	}

	// Combination of RPC ID - User ID
	key := strconv.FormatUint(rpcID, 10) + "-" + userID

	if checkKeyExists(key) {
		// Dont allow, it's a duplicated requests
		return false
	} else {

		// TODO: Check if key exists. KEY must be unique!
		protocolConnections[key] = ProtocolRequest{
			Protocol:      protocol,
			RPCID:         rpcID,
			UserID:        userID,
			RequestedTime: uint64(time.Now().UnixMicro()),
			ws:            ws,
			gin:           gin,
		}
		return true
	}
}

func UnregisterProtocol(rpcID uint64, userID string) {
	// Combination of RPC ID - User ID
	key := strconv.FormatUint(rpcID, 10) + "-" + userID

	if protocolConnections != nil {
		delete(protocolConnections, key)
	}
}

func GetProtocol(rpcID uint64, userID string) (bool, ProtocolRequest) {
	// Combination of RPC ID - User ID
	key := strconv.FormatUint(rpcID, 10) + "-" + userID

	val, ok := protocolConnections[key]

	if ok {
		return true, val
	}
	return false, ProtocolRequest{}
}

// Responsible for constructing the error message
func SendErrorMessage(rpcID uint64, userID string, validation int, payload interface{}) bool {

	// Construct the response structure
	reason := validation_reason.ValidationReason(validation)
	code, codeStr := reason.Code()
	var m RPCResponseMessage
	m = RPCResponseMessage{
		Result:  payload,
		JSONRPC: "2.0",
		ID:      rpcID,
		Testnet: true,
	}
	m.Error = ErrorMessage{
		Message: reason.String(),
		Data: ReasonMessage{
			Reason: codeStr,
		},
		Code: code,
	}

	return doSend(rpcID, userID, m)

}

// Responsible for constructing the message
func SendMessage(rpcID uint64, userID string, payload interface{}) bool {

	// Construct the response structure
	var m RPCResponseMessage
	m = RPCResponseMessage{
		Result:  payload,
		JSONRPC: "2.0",
		ID:      rpcID,
		Testnet: true,
	}

	return doSend(rpcID, userID, m)

}

// Responsible for handling to send for different protocol
func doSend(rpcID uint64, userID string, m RPCResponseMessage) bool {

	ok, val := GetProtocol(rpcID, userID)
	if !ok {
		return false
	}

	m.UsIn = val.RequestedTime
	m.UsOut = uint64(time.Now().UnixMicro())
	m.UsDiff = m.UsOut - m.UsIn

	// Websocket
	if val.Protocol == Websocket {
		// val.ws.SendMessageRaw(m) TODO

		UnregisterProtocol(rpcID, userID)
	}

	// REST API
	if val.Protocol == POST || val.Protocol == GET {
		val.gin.JSON(http.StatusAccepted, m)
		UnregisterProtocol(rpcID, userID)
	}

	// TODO: GRPC?
	return true
}
