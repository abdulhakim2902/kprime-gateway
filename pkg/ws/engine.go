package ws

import (
	"errors"
	"sync"
)

var engine *EngineSocket

// Engine holds the map of subscribtions subscribed to pair channels
// corresponding to the key/event they have subscribed to.
type EngineSocket struct {
	subscriptions     map[string]map[*Client]bool
	subscriptionsList map[*Client][]string
	mu                sync.Mutex
}

func NewEngineSocket() *EngineSocket {
	return &EngineSocket{
		subscriptions:     make(map[string]map[*Client]bool),
		subscriptionsList: make(map[*Client][]string),
		mu:                sync.Mutex{},
	}
}

// GetEngineSocket return singleton instance of PairSockets type struct
func GetEngineSocket() *EngineSocket {
	if engine == nil {
		engine = NewEngineSocket()
	}

	return engine
}

// Subscribe handles the subscription of connection to get
// streaming data over the socker for any pair.
// pair := utils.GetPairKey(bt, qt)
func (s *EngineSocket) Subscribe(channelID string, c *Client) error {
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
func (s *EngineSocket) UnsubscribeHandler(channelID string) func(c *Client) {
	return func(c *Client) {
		s.UnsubscribeChannel(channelID, c)
	}
}

// Unsubscribe is used to unsubscribe the connection from listening to the key
// subscribed to. It can be called on unsubscription message from user or due to some other reason by
// system
func (s *EngineSocket) UnsubscribeChannel(channelID string, c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.subscriptions[channelID][c] {
		s.subscriptions[channelID][c] = false
		delete(s.subscriptions[channelID], c)
	}
}

func (s *EngineSocket) Unsubscribe(c *Client) {
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
func (s *EngineSocket) BroadcastMessage(channelID string, p interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for c, status := range s.subscriptions[channelID] {
		if status {
			s.SendUpdateMessage(c, p)
		}
	}

	return nil
}

// SendErrorMessage sends error message on orderbookchannel
// First Element In Params Is Requested ID
func (s *EngineSocket) SendErrorMessage(c *Client, data interface{}, params ...uint64) {
	c.SendMessage(data, params[0])
}

// SendInitMessage sends INIT message on orderbookchannel on subscription event
// First Element In Params Is Requested ID
func (s *EngineSocket) SendInitMessage(c *Client, data interface{}, params ...uint64) {
	c.SendMessage(data, params[0])
}

// SendUpdateMessage sends UPDATE message on enginechannel as new data is created
// First Element In Params Is Requested ID
func (s *EngineSocket) SendUpdateMessage(c *Client, data interface{}, params ...uint64) {
	c.SendMessage(data, params[0])
}
