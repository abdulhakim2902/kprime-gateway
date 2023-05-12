package secmaster

import (
	"github.com/quickfixgo/enum"

	"github.com/quickfixgo/quickfix"
)

// SecurityDefinitionRequest is the SecurityDefinitionRequest type
type MarketDataRequest struct {
	ID                      int                      `json:"id"`
	SessionID               quickfix.SessionID       `json:"-"`
	Session                 string                   `json:"session_id"`
	SecurityRequestType     enum.SecurityRequestType `json:"security_request_type"`
	Symbol                  string                   `json:"symbol"`
	SubscriptionRequestType string                   `json:"subscription_request_type"`
	Ask                     string                   `json:"md_entry_type_1"`
	Bid                     string                   `json:"md_entry_type_2"`
	Trade                   string                   `json:"md_entry_type_3"`
}
