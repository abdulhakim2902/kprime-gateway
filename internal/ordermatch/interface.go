package ordermatch

import "github.com/quickfixgo/quickfix"

type IApplication interface {
	//Notification of a session begin created.
	OnCreate(sessionID quickfix.SessionID)

	//Notification of a session successfully logging on.
	OnLogon(sessionID quickfix.SessionID)

	//Notification of a session logging off or disconnecting.
	OnLogout(sessionID quickfix.SessionID)

	//Notification of admin message being sent to target.
	ToAdmin(message quickfix.Message, sessionID quickfix.SessionID)

	//Notification of app message being sent to target.
	ToApp(message quickfix.Message, sessionID quickfix.SessionID) error

	//Notification of admin message being received from target.
	FromAdmin(message quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError

	//Notification of app message being received from target.
	FromApp(message quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError
}
