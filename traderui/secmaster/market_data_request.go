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
	MdEntryTypes            string                   `json:"md_entry_types"`
	Ask                     bool                     `json:"md_entry_type_2"`
	Bid                     bool                     `json:"md_entry_type_1"`
	Trade                   bool                     `json:"md_entry_type_3"`
}

type MassCancelRequest struct {
	Symbol     string             `json:"symbol"`
	Session    string             `json:"session"`
	SessionID  quickfix.SessionID `json:"-"`
	CancelType string             `json:"cancel_type"`
}
