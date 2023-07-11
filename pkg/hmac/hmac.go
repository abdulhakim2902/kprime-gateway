package hmac

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/gin-gonic/gin"
)

type Hmac struct{}

func New() Hmac {
	return Hmac{}
}

func (h *Hmac) DecodeSignature(signature string, c *gin.Context) (sign Signature, err error) {
	signatures := strings.Split(signature, ",")
	if len(signatures) != 4 {
		err = errors.New("signature length invalid")
		logs.Log.Error().Err(err).Msg("")
		return
	}

	clientId, ok := getSignatureValue("id", signatures[0])
	if !ok {
		err = errors.New("signature id not invalid")

		logs.Log.Error().Err(err).Msg("")
		return
	}

	ts, ok := getSignatureValue("ts", signatures[1])
	if !ok {
		err = errors.New("signature ts not invalid")

		logs.Log.Error().Err(err).Msg("")
		return
	}

	sig, ok := getSignatureValue("sig", signatures[2])
	if !ok {
		err = errors.New("signature sig not invalid")

		logs.Log.Error().Err(err).Msg("")
		return
	}

	nonce, ok := getSignatureValue("nonce", signatures[3])
	if !ok {
		err = errors.New("signature nonce not invalid")

		logs.Log.Error().Err(err).Msg("")
		return
	}

	// Data
	body, _ := c.Get("body")
	bodyStr := ""
	if b, ok := body.([]byte); ok {
		bodyStr = string(b)
	}

	data := fmt.Sprintf("%s\n%s\n%s\n", c.Request.Method, c.Request.RequestURI, bodyStr)
	data = fmt.Sprintf("%s\n%s\n%s", ts, nonce, data)

	sign = Signature{
		ClientId: clientId,
		Ts:       ts,
		Sig:      sig,
		Nonce:    nonce,
		Data:     data,
	}

	return
}

func getSignatureValue(key, str string) (string, bool) {
	if strings.HasPrefix(str, fmt.Sprintf("%s=", key)) {
		splitted := strings.Split(str, "=")

		return splitted[1], true
	}

	return "", false
}
