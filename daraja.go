package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
)

func getDarajaCredentials() (string, string) {
	consumerKey := os.Getenv("DARAJA_CONSUMER_KEY")
	consumerSecret := os.Getenv("DARAJA_CONSUMER_SECRET")
	return consumerKey, consumerSecret
}
func getAccessToken(consumerKey, consumerSecret string) (string, error) {
	url := "https://sandbox.safaricom.co.ke/oauth/v1/generate?grant_type=client_credentials"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	credentials := consumerKey + ":" + consumerSecret
	encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
	req.Header.Set("Authorization", "Basic "+encoded)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}

	return result.AccessToken, nil
}

	

