package protocol

import (
	"context"
	"errors"
	"fmt"
	"gateway/pkg/collector"
	"gateway/pkg/constant"
	"gateway/pkg/utils"
	"gateway/pkg/ws"
	"net/http"
	"sync"
	"time"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/Undercurrent-Technologies/kprime-utilities/types/validation_reason"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

type ProtocolType int

const (
	Websocket ProtocolType = iota
	HTTP
	GRPC
	Channel
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
	Method        string
}

var protocolConnections map[any]ProtocolRequest
var protocolMutex sync.RWMutex
var channelConnections map[any]chan RPCResponseMessage
var channelMutex sync.RWMutex
var channelResults map[any]RPCResponseMessage

// var contextStopMutex sync.RWMutex
var channelContextStop utils.Map

type stoper chan interface{}

var resultMutex sync.RWMutex
var channelTimeout map[string]chan bool
var timeoutMutex sync.RWMutex

func (p *ProtocolRequest) getcollectorProtocol() collector.Protocol {
	var protocol collector.Protocol
	if p.WS != nil {
		protocol = collector.WS
	} else if p.Http != nil {
		switch p.Http.Request.Method {
		case "POST":
			protocol = collector.HTTP_POST
			break
		case "GET":
			protocol = collector.HTTP_GET
			break
		}
	}

	return protocol
}

func RegisterProtocolRequest(key string, conn ProtocolRequest) (duplicateConnection bool) {
	protocolMutex.Lock()
	if protocolConnections == nil {
		protocolConnections = make(map[any]ProtocolRequest)
	}
	protocolMutex.Unlock()

	duplicateConnection = isConnExist(key)

	conn.RequestedTime = uint64(time.Now().UnixMicro())
	protocolMutex.Lock()
	protocolConnections[key] = conn
	protocolMutex.Unlock()

	// collector
	if !duplicateConnection {
		label := prometheus.Labels{
			"protocol": string(conn.getcollectorProtocol()),
			"method":   conn.Method,
		}

		go func(label prometheus.Labels) {
			collector.IncomingCounter.With(label).Inc()
		}(label)
	}

	return
}

func UpgradeProtocol(oldKey, newKey string) (duplicateConnection bool) {
	protocolMutex.RLock()
	conn := protocolConnections[oldKey]
	protocolMutex.RUnlock()

	duplicateConnection = isConnExist(newKey)
	// Set the new ID
	protocolMutex.Lock()
	protocolConnections[newKey] = conn

	// Remove old connection
	delete(protocolConnections, oldKey)
	protocolMutex.Unlock()

	return
}

func TimeOutProtocol(key string) {
	ticker := time.NewTicker(constant.TIMEOUT)
	timeoutMutex.Lock()
	if channelTimeout == nil {
		channelTimeout = make(map[string]chan bool)
	}
	channelTimeout[key] = make(chan bool)
	timeoutMutex.Unlock()

timeoutTicker:
	for {
		select {
		case <-channelTimeout[key]:
			ticker.Stop()
			timeoutMutex.Lock()
			delete(channelTimeout, key)
			timeoutMutex.Unlock()
			break timeoutTicker
		case <-ticker.C:
			ticker.Stop()
			timeoutMutex.Lock()
			delete(channelTimeout, key)
			timeoutMutex.Unlock()
			err := errors.New(validation_reason.TIME_OUT.String())
			SendValidationMsg(key, validation_reason.TIME_OUT, err)
			break timeoutTicker
		}
	}
}

func UnregisterProtocol(key string) {
	protocolMutex.Lock()
	if protocolConnections != nil {
		delete(protocolConnections, key)
	}
	protocolMutex.Unlock()
}

func UnregisterChannel(key string) {
	reported := make(stoper)
	channelContextStop.Store(key, reported)
}

func GetProtocol(key string) (bool, ProtocolRequest) {
	protocolMutex.RLock()
	defer protocolMutex.RUnlock()
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
	ok, conn := GetProtocol(key)
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

	m.UsIn = conn.RequestedTime
	m.UsOut = uint64(time.Now().UnixMicro())
	m.UsDiff = m.UsOut - m.UsIn
	switch conn.Protocol {
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

		conn.WS.Send(msg)

		timeoutMutex.RLock()
		timeout := channelTimeout[key]
		timeoutMutex.RUnlock()
		if timeout != nil {
			channelTimeout[key] <- true
		}

		break
	case HTTP:
		statusCode := http.StatusOK
		if m.Error != nil {
			statusCode = m.Error.HttpStatusCode
		}
		conn.Http.JSON(statusCode, m)

		break
	case GRPC:
		// TODO: add grpc response
		break
	case Channel:
		resultMutex.Lock()
		if channelResults == nil {
			channelResults = make(map[any]RPCResponseMessage)
		}
		channelResults[key] = m
		resultMutex.Unlock()
		break
	}

	// collector
	collectorLabel := prometheus.Labels{
		"protocol": string(conn.getcollectorProtocol()),
		"method":   conn.Method,
	}

	go func(label prometheus.Labels, errMsg *ErrorMessage, usDiff uint64) {
		if errMsg != nil {
			reason := validation_reason.PARSE_ERROR
			if errMsg.Message == reason.String() {
				collector.ErrorCounter.With(label).Inc()
			} else {
				collector.ValidationCounter.With(label).Inc()
			}

			collector.RequestDurationHistogram.WithLabelValues("False").Observe(float64(usDiff))
		} else {
			collector.SuccessCounter.With(label).Inc()
			collector.RequestDurationHistogram.WithLabelValues("True").Observe(float64(usDiff))
		}

	}(collectorLabel, m.Error, m.UsDiff)

	UnregisterProtocol(key)

	return true
}

func isConnExist(key string) bool {
	protocolMutex.RLock()
	defer protocolMutex.RUnlock()
	_, ok := protocolConnections[key]
	return ok
}

func RegisterChannel(key string, channel chan RPCResponseMessage, ctx context.Context) {
	channelMutex.Lock()
	if channelConnections == nil {
		channelConnections = make(map[any]chan RPCResponseMessage)
	}
	channelConnections[key] = channel
	channelMutex.Unlock()

	res := RPCResponseMessage{}
readChannel:
	for {
		resultMutex.Lock()
		res = channelResults[key]
		resultMutex.Unlock()
		if res.Result != nil || res.Error != nil {
			break
		}

		select {
		case <-ctx.Done():
			res = RPCResponseMessage{
				Error: &ErrorMessage{
					Message: validation_reason.TIME_OUT.String(),
					Data: ReasonMessage{
						Reason: validation_reason.TIME_OUT.String(),
					},
				},
			}
			break readChannel
		case <-channelContextStop.Load(key):
			break readChannel
		default:
			continue
		}
	}

	// Delete object from map after reading
	resultMutex.Lock()
	delete(channelResults, key)
	resultMutex.Unlock()
	channelMutex.Lock()
	delete(channelConnections, key)
	channelMutex.Unlock()

	channel <- res
}
