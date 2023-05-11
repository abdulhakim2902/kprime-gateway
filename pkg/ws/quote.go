package ws

import (
	"errors"
	"sync"
)

var quote *QuoteSocket

// QuoteSocket holds the map of subscribtions subscribed to pair channels
// corresponding to the key/event they have subscribed to.
type QuoteSocket struct {
	subscriptions     map[string]map[*Client]bool
	subscriptionsList map[*Client][]string
	mu                sync.Mutex
}

func NewQuoteSocket() *QuoteSocket {
	return &QuoteSocket{
		subscriptions:     make(map[string]map[*Client]bool),
		subscriptionsList: make(map[*Client][]string),
		mu:                sync.Mutex{},
	}
}

// GetQuoteSocket return singleton instance of PairSockets type struct
func GetQuoteSocket() *QuoteSocket {
	if quote == nil {
		quote = NewQuoteSocket()
	}

	return quote
}

// Subscribe handles the subscription of connection to get
// streaming data over the socker for any pair.
// pair := utils.GetPairKey(bt, qt)
func (s *QuoteSocket) Subscribe(channelID string, c *Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if c == nil {
		return errors.New("No connection found")
	}

	if s.subscriptions[channelID] == nil {
		s.subscriptions[channelID] = make(map[*Client]bool)
	}

	s.subscriptions[channelID][c] = true

	if s.subscriptionsList[c] == nil {
		s.subscriptionsList[c] = []string{}
	}

	s.subscriptionsList[c] = append(s.subscriptionsList[c], channelID)
	return nil
}

// UnsubscribeHandler returns function of type unsubscribe handler,
// it handles the unsubscription of pair in case of connection closing.
func (s *QuoteSocket) UnsubscribeHandler(channelID string) func(c *Client) {
	return func(c *Client) {
		s.UnsubscribeChannel(channelID, c)
	}
}

// Unsubscribe is used to unsubscribe the connection from listening to the key
// subscribed to. It can be called on unsubscription message from user or due to some other reason by
// system
func (s *QuoteSocket) UnsubscribeChannel(channelID string, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.subscriptions[channelID][c] {
		s.subscriptions[channelID][c] = false
		delete(s.subscriptions[channelID], c)
	}
}

func (s *QuoteSocket) Unsubscribe(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	channelIDs := s.subscriptionsList[c]
	if channelIDs == nil {
		return
	}

	for _, id := range s.subscriptionsList[c] {
		if s.subscriptions[id][c] {
			s.subscriptions[id][c] = false
			delete(s.subscriptions[id], c)
		}
	}
}

// BroadcastMessage streams message to all the subscribtions subscribed to the pair
func (s *QuoteSocket) BroadcastMessage(channelID string, method string, p interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for c, status := range s.subscriptions[channelID] {
		if status {
			s.SendUpdateMessage(c, method, p)
		}
	}

	return nil
}

// SendErrorMessage sends error message on orderbookchannel
func (s *QuoteSocket) SendErrorMessage(c *Client, data interface{}) {
	c.SendMessage(data, SendMessageParams{})
}

// SendInitMessage sends INIT message on orderbookchannel on subscription event
func (s *QuoteSocket) SendInitMessage(c *Client, method string, data interface{}) {
	c.SendMessageSubcription(data, method, SendMessageParams{})
}

// SendUpdateMessage sends UPDATE message on orderbookchannel as new data is created
func (s *QuoteSocket) SendUpdateMessage(c *Client, method string, data interface{}) {
	c.SendMessageSubcription(data, method, SendMessageParams{})
}
