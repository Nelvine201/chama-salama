package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
	"bytes"
)

const (
	darajaShortcode = "174379"
	darajaPasskey   = "bfb279f9aa9bdbcf158e97dd71a467cd2e0c893059b10f78e6b72ada1ed2c919"
)
func generateStkPassword() (string, string) {
	timestamp := time.Now().Format("20060102150405")
	raw := darajaShortcode + darajaPasskey + timestamp
	password := base64.StdEncoding.EncodeToString([]byte(raw))
	return password, timestamp
}

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
func sendStkPush(accessToken, phone string, amount int) (string, error) {
	password, timestamp := generateStkPassword()

	payload := map[string]interface{}{
		"BusinessShortCode": darajaShortcode,
		"Password":          password,
		"Timestamp":         timestamp,
		"TransactionType":   "CustomerPayBillOnline",
		"Amount":            amount,
		"PartyA":            phone,
		"PartyB":            darajaShortcode,
		"PhoneNumber":       phone,
		"CallBackURL":       "https://handcart-latitude-decompose.ngrok-free.dev/callback",
		"AccountReference":  "ChamaSalama",
		"TransactionDesc":   "Chama contribution",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := "https://sandbox.safaricom.co.ke/mpesa/stkpush/v1/processrequest"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(respBody), nil
}

	

