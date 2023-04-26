package ws

import "fmt"

// OrderConn is websocket order connection struct
// It holds the reference to connection and the channel of type OrderMessage

type OrderConnection []*Client

var orderConnections map[string]OrderConnection

// GetOrderConn returns the connection associated with an order ID
func GetOrderConnections(a string) OrderConnection {
	c := orderConnections[a]
	if c == nil {
		// logger.Warning("No connection found")
		fmt.Println("No connection found")
		return nil
	}

	return orderConnections[a]
}

func OrderSocketUnsubscribeHandler(a string) func(client *Client) {
	return func(client *Client) {
		// logger.Info("In unsubscription handler")
		orderConnection := orderConnections[a]
		if orderConnection == nil {
			// logger.Info("No subscriptions")
		}

		if orderConnection != nil {
			// logger.Info("%v connections before unsubscription", len(orderConnections[a]))
			for i, c := range orderConnection {
				if client == c {
					orderConnection = append(orderConnection[:i], orderConnection[i+1:]...)
				}
			}

		}

		orderConnections[a] = orderConnection
		// logger.Info("%v connections after unsubscription", len(orderConnections[a]))
	}
}

// RegisterOrderConnection registers a connection with and orderID.
// It is called whenever a message is recieved over order channel
func RegisterOrderConnection(a string, c *Client) {
	// logger.Info("Registering new order connection")

	if orderConnections == nil {
		orderConnections = make(map[string]OrderConnection)
	}

	if orderConnections[a] == nil {
		// logger.Info("Registering a new order connection")
		orderConnections[a] = OrderConnection{c}
		RegisterConnectionUnsubscribeHandler(c, OrderSocketUnsubscribeHandler(a))
		// logger.Info("Number of connections for this address: %v", len(orderConnections))
	}

	if orderConnections[a] != nil {
		if !isClientConnected(a, c) {
			// logger.Info("Registering a new order connection")
			orderConnections[a] = append(orderConnections[a], c)
			RegisterConnectionUnsubscribeHandler(c, OrderSocketUnsubscribeHandler(a))
			// logger.Info("Number of connections for this address: %v", len(orderConnections))
		}
	}
}

func isClientConnected(a string, client *Client) bool {
	for _, c := range orderConnections[a] {
		if c == client {
			// logger.Info("Client is connected")
			return true
		}
	}

	// logger.Info("Client is not connected")
	return false
}

func SendOrderMessage(a string, payload interface{}, params SendMessageParams) {
	conn := GetOrderConnections(a)
	if conn == nil {
		return
	}

	for _, c := range conn {
		c.SendMessage(payload, params)
	}
}
