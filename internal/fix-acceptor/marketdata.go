package ordermatch

import (
	"github.com/quickfixgo/quickfix"
)

func VSubscribe(symbol string, sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	if subsManager.VSubscriptions[symbol] == nil {
		subsManager.VSubscriptions[symbol] = make(map[quickfix.SessionID]bool)
	}

	subsManager.VSubscriptions[symbol][sessionID] = true

	if subsManager.VSubscriptionsList[sessionID] == nil {
		subsManager.VSubscriptionsList[sessionID] = []string{}
	}

	subsManager.VSubscriptionsList[sessionID] = append(subsManager.VSubscriptionsList[sessionID], symbol)
}

func VUnsubscribeAll(sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	symbols := subsManager.VSubscriptionsList[sessionID]
	if symbols == nil {
		return
	}

	for _, id := range subsManager.VSubscriptionsList[sessionID] {
		if subsManager.VSubscriptions[id][sessionID] {
			subsManager.VSubscriptions[id][sessionID] = false
			delete(subsManager.VSubscriptions[id], sessionID)
		}
	}
}

func VUnsubscribe(symbol string, sessionID quickfix.SessionID) {
	subsManager.mu.Lock()
	defer subsManager.mu.Unlock()

	if subsManager.VSubscriptions[symbol][sessionID] {
		subsManager.VSubscriptions[symbol][sessionID] = false
		delete(subsManager.VSubscriptions[symbol], sessionID)
	}
}
