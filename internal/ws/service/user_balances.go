package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	deribitModel "gateway/internal/deribit/model"

	"git.devucc.name/dependencies/utilities/commons/logs"
)

type wsUserBalanceService struct {
}

func NewWSUserBalanceService() IwsUserBalanceService {
	return &wsUserBalanceService{}
}

func (svc wsUserBalanceService) FetchUserBalance(currency string, userID string) deribitModel.GetAccountSummaryResult {
	url := fmt.Sprintf("%s/api/v1/users/%s/balances", os.Getenv("MATCHING_ENGINE_URL"), userID)
	var auth string

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logs.Log.Err(err).Msg("Error creating HTTP request!")
		return deribitModel.GetAccountSummaryResult{}
	}

	token, _ := os.LookupEnv("PROTECT_BASIC_ENGINE")
	if token != "*" {
		auth = fmt.Sprintf("Basic %s", token)
		req.Header.Add("Authorization", auth)
	}

	res, err := client.Do(req)
	if err != nil {
		logs.Log.Err(err).Msg("Invalid url!")
		return deribitModel.GetAccountSummaryResult{}
	}

	if res.StatusCode != 200 {
		logs.Log.Error().Msg(res.Status)
		return deribitModel.GetAccountSummaryResult{}
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logs.Log.Err(err).Msg("Failed to read data!")
		return deribitModel.GetAccountSummaryResult{}
	}

	result := deribitModel.GetAccountSummaryRes{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		logs.Log.Err(err).Msg("Failed to decode data!")
		return deribitModel.GetAccountSummaryResult{}
	}

	resp := deribitModel.GetAccountSummaryResult{}
	for _, balance := range result.Data {
		if balance.Currency == currency {
			resp.Balance = balance.Balance
			resp.Currency = balance.Currency
			resp.UserId = balance.UserId
		}
	}

	return resp
}
