package protocol

import (
	"fmt"
	"gateway/pkg/utils"
	"gateway/pkg/ws"
	"net/http"
	"time"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/types/validation_reason"
	"github.com/gin-gonic/gin"
)

type ProtocolType int

const (
	Websocket ProtocolType = iota
	HTTP
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
	Error *ErrorMessage `json:"error,omitempty"`
}

type ErrorMessage struct {
	Message string        `json:"message"`
	Data    ReasonMessage `json:"data"`
	Code    int64         `json:"code"`

	// Helper to pass the http status code
	HttpStatusCode int `json:"-"`
}

type ReasonMessage struct {
	Reason string `json:"reason"`
}

type ProtocolRequest struct {
	Protocol      ProtocolType
	RequestedTime uint64
	WS            *ws.Client
	Http          *gin.Context
}

var protocolConnections map[any]ProtocolRequest

func RegisterProtocolRequest(key string, conn ProtocolRequest) (duplicateConnection bool) {
	if protocolConnections == nil {
		protocolConnections = make(map[any]ProtocolRequest)
	}

	duplicateConnection = isConnExist(key)

	conn.RequestedTime = uint64(time.Now().UnixMicro())
	protocolConnections[key] = conn

	return
}

func UpgradeProtocol(oldKey, newKey string) bool {
	conn := protocolConnections[oldKey]

	// Set the new ID
	protocolConnections[newKey] = conn

	// Remove old connection
	delete(protocolConnections, oldKey)

	return true
}

func UnregisterProtocol(key string) {
	if protocolConnections != nil {
		delete(protocolConnections, key)
	}
}

func GetProtocol(key string) (bool, ProtocolRequest) {
	val, ok := protocolConnections[key]
	if ok {
		return true, val
	}

	return false, ProtocolRequest{}
}

// Responsible for constructing the message
func SendSuccessMsg(key string, result any) bool {
	return doSend(key, result, nil)
}

// Responsible for constructing the validation message
func SendValidationMsg(key string, reason validation_reason.ValidationReason, err error) bool {
	reasongMsg := reason.String()
	if err != nil {
		reasongMsg = err.Error()
	}

	code, httpCode, codeStr := reason.Code()
	errMsg := ErrorMessage{
		Message: reasongMsg,
		Data: ReasonMessage{
			Reason: codeStr,
		},
		Code:           code,
		HttpStatusCode: httpCode,
	}

	logs.Log.Debug().Str("validation_reason", codeStr).Msg(reasongMsg)

	return doSend(key, nil, &errMsg)
}

// Responsible for constructing the error message
func SendErrMsg(key string, err error) bool {
	reason := validation_reason.OTHER

	code, httpCode, codeStr := reason.Code()
	errMsg := ErrorMessage{
		Message: reason.String(),
		Data: ReasonMessage{
			Reason: codeStr,
		},
		Code:           code,
		HttpStatusCode: httpCode,
	}

	return doSend(key, nil, &errMsg)
}

// Responsible for handling to send for different protocol
func doSend(key string, result any, err *ErrorMessage) bool {
	ok, val := GetProtocol(key)
	if !ok {
		logs.Log.Error().Str("connection_key", key).Msg("no connection found")

		return false
	}

	logs.Log.Info().Str("connection_key", key).Msg("protocol send message")

	m := RPCResponseMessage{
		JSONRPC: "2.0",
		Result:  result,
		Error:   err,
		Testnet: true,
	}

	ID, _ := utils.GetIdUserIDFromKey(fmt.Sprintf("%v", key))
	m.ID = ID

	m.UsIn = val.RequestedTime
	m.UsOut = uint64(time.Now().UnixMicro())
	m.UsDiff = m.UsOut - m.UsIn

	switch val.Protocol {
	case Websocket:
		msg := ws.WebsocketResponseMessage{
			JSONRPC: m.JSONRPC,
			ID:      m.ID,
			Method:  m.Method,
			Result:  m.Result,
			Testnet: m.Testnet,
			UsIn:    m.UsIn,
			UsOut:   m.UsOut,
			UsDiff:  m.UsDiff,
		}

		if m.Error != nil {
			msg.Error = m.Error
		}

		val.WS.Send(msg)

		break
	case HTTP:
		statusCode := http.StatusOK
		if m.Error != nil {
			statusCode = m.Error.HttpStatusCode
		}
		val.Http.JSON(statusCode, m)

		break
	case GRPC:
		// TODO: add grpc response
		break
	}

	UnregisterProtocol(key)

	return true
}

func isConnExist(key string) bool {
	_, ok := protocolConnections[key]
	return ok
}
