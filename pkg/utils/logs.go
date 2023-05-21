package utils

import (
	"os"

	"git.devucc.name/dependencies/utilities/commons/logs"
)

const (
	GATEWAY logs.LoggerType = "GATEWAY"
)

func InitLogger() {
	logs.InitLogger(GATEWAY)

	// Enable discord logs if DISLOG_WEBHOOK_URL is provided
	if len(os.Getenv("DISLOG_WEBHOOK_URL")) > 0 {
		logs.WithDiscord(os.Getenv("DISLOG_WEBHOOK_URL"))
	}
}
