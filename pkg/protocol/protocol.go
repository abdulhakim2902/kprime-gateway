package protocol

import (
	"fmt"
	"gateway/pkg/utils"
	"gateway/pkg/ws"
	"net/http"
	"strconv"
	"strings"
	"sync"
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
	HttpStatusCode int
}

type ReasonMessage struct {
	Reason string `json:"reason"`
}

type ProtocolRequest struct {
	Protocol      ProtocolType
	ID            string
	RequestedTime uint64
	ws            *ws.Client
	gin           *gin.Context
}

var protocolConnections map[any]ProtocolRequest
var protocolMutex sync.RWMutex

func RegisterProtocolRequest(
	ID any,
	protocol ProtocolType,
	ws *ws.Client,
	gin *gin.Context,
) bool {
	protocolMutex.Lock()
	if protocolConnections == nil {
		protocolConnections = make(map[any]ProtocolRequest)
	}
	protocolMutex.Unlock()

	if checkKeyExists(ID) {
		return false
	}
	protocolMutex.Lock()
	protocolConnections[ID] = ProtocolRequest{
		Protocol:      protocol,
		RequestedTime: uint64(time.Now().UnixMicro()),
		ws:            ws,
		gin:           gin,
	}
	protocolMutex.Unlock()

	return true
}

func UpgradeProtocol(OldID, NewID any) bool {
	protocolMutex.Lock()
	defer protocolMutex.Unlock()
	conn := protocolConnections[OldID]

	// Set the new ID
	protocolConnections[NewID] = conn

	// Remove old connection
	delete(protocolConnections, OldID)

	return true
}

func UnregisterProtocol(ID any) {
	protocolMutex.Lock()
	if protocolConnections != nil {
		delete(protocolConnections, ID)
	}
	protocolMutex.Unlock()
}

func GetProtocol(ID any) (bool, ProtocolRequest) {
	protocolMutex.RLock()
	defer protocolMutex.RUnlock()
	val, ok := protocolConnections[ID]
	if ok {
		return true, val
	}

	return false, ProtocolRequest{}
}

// Responsible for constructing the message
func SendSuccessMsg(ID any, result any) bool {
	return doSend(ID, result, nil)
}

// Responsible for constructing the validation message
func SendValidationMsg(ID any, reason validation_reason.ValidationReason, err error) bool {

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

	return doSend(ID, nil, &errMsg)
}

// Responsible for constructing the error message
func SendErrMsg(ID any, err error) bool {
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

	return doSend(ID, nil, &errMsg)
}

// Responsible for handling to send for different protocol
func doSend(ID any, result any, err *ErrorMessage) bool {
	ok, val := GetProtocol(ID)
	if !ok {
		logs.Log.Error().Any("ID", ID).Msg("no connection found")

		return false
	}

	logs.Log.Info().Any("ID", ID).Msg("protocol send message")

	s := fmt.Sprintf("%d", ID)
	msgId, _ := strconv.ParseInt(s, 10, 64)

	m := RPCResponseMessage{
		JSONRPC: "2.0",
		Result:  result,
		Error:   err,
		Testnet: true,
		ID:      uint64(msgId),
	}

	id := fmt.Sprintf("%v", ID)
	if strings.Contains(id, "-") {
		rpcID, _ := utils.ParseKey(id)
		m.ID = rpcID
	}

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

		val.ws.Send(msg)

		break
	case HTTP:
		statusCode := http.StatusOK
		if m.Error != nil {
			statusCode = m.Error.HttpStatusCode
		}
		val.gin.JSON(statusCode, m)

		break
	case GRPC:
		// TODO: add grpc response
		break
	}

	UnregisterProtocol(ID)

	return true
}

func checkKeyExists(key any) bool {
	protocolMutex.RLock()
	defer protocolMutex.RUnlock()
	_, ok := protocolConnections[key]
	return ok
}
