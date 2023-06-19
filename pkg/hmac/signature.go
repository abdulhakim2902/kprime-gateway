package hmac

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"git.devucc.name/dependencies/utilities/commons/logs"
)

type Signature struct {
	Ts, ClientId, Sig, Nonce, Data string
}

func (s *Signature) GenerateMessage(key string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(s.Data))

	return mac.Sum(nil)
}

func (s *Signature) Sign(key string) string {
	return hex.EncodeToString(s.GenerateMessage(key))
}

func (s *Signature) Verify(key string) bool {
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

	decoded, err := hex.DecodeString(s.Sig)
	if err != nil {
		logs.Log.Error().Err(err).Msg("")

		return false
	}

	msg := s.GenerateMessage(key)

	// Is signature ok
	signatureOk := hmac.Equal(decoded, msg)
	if !signatureOk {
		logs.Log.Warn().Msg("signature not ok")
	}

	return signatureOk

}
