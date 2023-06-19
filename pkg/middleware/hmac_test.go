package middleware

import (
	"encoding/json"
	"fmt"
	"gateway/pkg/utils"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var (
	testKey = "yek-terces-tluafed"
	sig     = Signature{
		Ts:       "1686730272930",
		ClientId: "clientId",
		Nonce:    "nonce",
	}
)

func TestGenerateAndSignMessage(t *testing.T) {

	hash := sig.Sign(testKey)

	expectedHash := "014124da1c04cbd6bc65b89d88f6feb8df849b1a1e456a3a6bc1ea0c24891519"
	assert.Equal(t, expectedHash, hash)

}

func TestGetSignatureValue(t *testing.T) {
	val, ok := getSignatureValue("id", "id=id")
	assert.Equal(t, true, ok, "Should ok")
	assert.Equal(t, "id", val, "Should get id")
}

func TestGetRequest(t *testing.T) {
	utils.InitLogger()
	ctx, engine := gin.CreateTestContext(httptest.NewRecorder())
	engine.GET("/test", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "Hello")
	})

	var err error
	ctx.Request, err = http.NewRequest(http.MethodGet, "/test", nil)
	if err != nil {
		panic(err)
	}
	engine.HandleContext(ctx)

	// Manipulate request data
	path := ctx.Request.URL.Path
	querys := ctx.Request.URL.Query()
	if len(querys) > 0 {
		path = fmt.Sprintf("%s?%s", path, querys.Encode())
	}

	b := strings.Join([]string{ctx.Request.Method, path, "", "\n"}, "\n")
	sig.Ts = strconv.Itoa(int(time.Now().UnixMilli()))
	data := strings.Join([]string{sig.Ts, sig.Nonce, b}, "\n")
	sig.Data = data
	hash := sig.Sign(testKey)

	// Test Wrong signature
	hmac := NewHmac()
	signature := fmt.Sprintf("id=%s,ts=%s,sig=%s,nonce=%s", sig.ClientId, sig.Ts, "wrong hash", sig.Nonce)
	decodedSig, err := hmac.DecodeSignature(signature, ctx)
	assert.NoError(t, err, "Should not error")

	assert.Equal(t, sig.ClientId, decodedSig.ClientId, "Client id check")
	assert.Equal(t, sig.Ts, decodedSig.Ts, "Ts id check")
	assert.Equal(t, sig.Nonce, decodedSig.Nonce, "Nonce id check")

	ok := decodedSig.Verify(testKey)
	assert.Equal(t, false, ok, "Expected not ok")

	// Test correct signature
	signature = fmt.Sprintf("id=%s,ts=%s,sig=%s,nonce=%s", sig.ClientId, sig.Ts, hash, sig.Nonce)
	decodedSig, err = hmac.DecodeSignature(signature, ctx)
	assert.NoError(t, err, "Should not error")

	assert.Equal(t, sig.ClientId, decodedSig.ClientId, "Client id check")
	assert.Equal(t, sig.Ts, decodedSig.Ts, "Ts id check")
	assert.Equal(t, sig.Nonce, decodedSig.Nonce, "Nonce id check")

	ok = decodedSig.Verify(testKey)
	assert.Equal(t, true, ok, "Expected ok")

}

func TestPostRequest(t *testing.T) {
	ctx, engine := gin.CreateTestContext(httptest.NewRecorder())
	engine.POST("/test", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "Hello")
	})

	payload := "{\"test\":\"value\"}"

	jsonPayload, err := json.Marshal(payload)
	assert.NoError(t, err, "should not error")

	ctx.Request, err = http.NewRequest(http.MethodPost, "/test", strings.NewReader(string(jsonPayload)))
	if err != nil {
		panic(err)
	}
	engine.HandleContext(ctx)

	ctx.Set("body", jsonPayload)

	// Manipulate request data
	path := ctx.Request.URL.Path
	querys := ctx.Request.URL.Query()
	if len(querys) > 0 {
		path = fmt.Sprintf("%s?%s", path, querys.Encode())
	}

	body, _ := ctx.Get("body")
	bodyStr := ""
	if b, ok := body.([]byte); ok {
		bodyStr = string(b)
	}

	b := strings.Join([]string{ctx.Request.Method, path, bodyStr, "\n"}, "\n")
	sig.Ts = strconv.Itoa(int(time.Now().UnixMilli()))
	data := strings.Join([]string{sig.Ts, sig.Nonce, b}, "\n")
	sig.Data = data
	hash := sig.Sign(testKey)

	// Test Wrong signature
	hmac := NewHmac()
	signature := fmt.Sprintf("id=%s,ts=%s,sig=%s,nonce=%s", sig.ClientId, sig.Ts, "wrong hash", sig.Nonce)
	decodedSig, err := hmac.DecodeSignature(signature, ctx)
	assert.NoError(t, err, "Should not error")

	assert.Equal(t, sig.ClientId, decodedSig.ClientId, "Client id check")
	assert.Equal(t, sig.Ts, decodedSig.Ts, "Ts id check")
	assert.Equal(t, sig.Nonce, decodedSig.Nonce, "Nonce id check")

	ok := decodedSig.Verify(testKey)
	assert.Equal(t, false, ok, "Expected not ok")

	// Test correct signature
	signature = fmt.Sprintf("id=%s,ts=%s,sig=%s,nonce=%s", sig.ClientId, sig.Ts, hash, sig.Nonce)
	decodedSig, err = hmac.DecodeSignature(signature, ctx)
	assert.NoError(t, err, "Should not error")

	assert.Equal(t, sig.ClientId, decodedSig.ClientId, "Client id check")
	assert.Equal(t, sig.Ts, decodedSig.Ts, "Ts id check")
	assert.Equal(t, sig.Nonce, decodedSig.Nonce, "Nonce id check")

	ok = decodedSig.Verify(testKey)
	assert.Equal(t, true, ok, "Expected ok")

}
