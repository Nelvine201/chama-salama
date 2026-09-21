package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
	"fmt"
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
type StkResponse struct {
	MerchantRequestID  string `json:"MerchantRequestID"`
	CheckoutRequestID  string `json:"CheckoutRequestID"`
	ResponseCode       string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
}

func sendStkPush(accessToken, phone string, amount int) (*StkResponse, error) {
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
		"CallBackURL":       "https://chama-salama.onrender.com/callback",
		"AccountReference":  "ChamaSalama",
		"TransactionDesc":   "Chama contribution",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := "https://sandbox.safaricom.co.ke/mpesa/stkpush/v1/processrequest"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println("Raw STK response:", string(respBody))

	var result StkResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if result.ResponseCode == "" {
		var errResp struct {
			ErrorCode    string `json:"errorCode"`
			ErrorMessage string `json:"errorMessage"`
		}
		json.Unmarshal(respBody, &errResp)
		if errResp.ErrorMessage != "" {
			return nil, fmt.Errorf("Daraja error [%s]: %s", errResp.ErrorCode, errResp.ErrorMessage)
		}
	}

	return &result, nil
}