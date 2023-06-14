package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"git.devucc.name/dependencies/utilities/commons/logs"
)

type Signature struct {
	Ts, ClientId, Sig, Nonce, Data string
}

func (s *Signature) GenerateMessage() []byte {
	if len(os.Getenv("HMAC_SECRET_KEY")) > 0 {
		key = []byte(os.Getenv("HMAC_SECRET_KEY"))
	}

	msg := fmt.Sprintf("%v\n%v\n%v", s.Ts, s.Nonce, s.Data)

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))

	return mac.Sum(nil)
}

func (s *Signature) Sign() string {
	return hex.EncodeToString(s.GenerateMessage())
}

func (s *Signature) Verify() bool {
	decoded, err := hex.DecodeString(s.Sig)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return false
	}

	msg := s.GenerateMessage()

	// Is signature ok
	signatureOk := hmac.Equal(decoded, msg)
	if !signatureOk {
		logs.Log.Warn().Msg("signature not ok")

		return false
	}

	ts, err := strconv.Atoi(s.Ts)
	if err != nil {
		logs.Log.Warn().Msg("ts not ok")

		return false
	}

	signatureTs := time.UnixMilli(int64(ts))
	now := time.Now()

	// Is timestamp ok
	if now.Sub(signatureTs) > time.Minute {
		logs.Log.Warn().Msg("signature expired")

		return false
	}

	return true

}
