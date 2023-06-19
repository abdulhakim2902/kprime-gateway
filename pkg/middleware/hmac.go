package middleware

import (
	"errors"
	"fmt"
	"strings"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"github.com/gin-gonic/gin"
)

type Hmac struct{}

func NewHmac() Hmac {
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
	path := c.Request.URL.Path
	querys := c.Request.URL.Query()
	if len(querys) > 0 {
		path = fmt.Sprintf("%s?%s", path, querys.Encode())
	}

	body, _ := c.Get("body")
	bodyStr := ""
	if b, ok := body.([]byte); ok {
		bodyStr = string(b)
	}

	b := strings.Join([]string{c.Request.Method, path, bodyStr, "\n"}, "\n")
	data := strings.Join([]string{ts, nonce, b}, "\n")

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
