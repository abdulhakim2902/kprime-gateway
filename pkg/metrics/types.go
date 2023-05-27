package metrics

type Protocol string

const (
	WS        Protocol = "ws"
	FIX       Protocol = "fix"
	HTTP_GET  Protocol = "rest_get"
	HTTP_POST Protocol = "rest_post"
)
