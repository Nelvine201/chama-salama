package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func darajaCallbackHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		defer r.Body.Close()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read callback", http.StatusBadRequest)
			return
		}

		fmt.Printf("Daraja callback received: body size=%d bytes\n", len(body))

		var payload struct {
			Body struct {
				StkCallback struct {
					CheckoutRequestID string `json:"CheckoutRequestID"`
					ResultCode        *int   `json:"ResultCode"`
					CallbackMetadata  struct {
						Items []struct {
							Name  string      `json:"Name"`
							Value interface{} `json:"Value"`
						} `json:"Item"`
					} `json:"CallbackMetadata"`
				} `json:"stkCallback"`
			} `json:"Body"`
		}

		if err := json.Unmarshal(body, &payload); err != nil {
			fmt.Printf("Daraja callback JSON error: %v\n", err)
			http.Error(w, "Invalid callback payload", http.StatusBadRequest)
			return
		}

		callback := payload.Body.StkCallback

		if strings.TrimSpace(callback.CheckoutRequestID) == "" || callback.ResultCode == nil {
			fmt.Println("Daraja callback rejected: missing required fields")
			http.Error(w, "Missing callback fields", http.StatusBadRequest)
			return
		}

		success := *callback.ResultCode == 0

		var amount float64
		var phone string
		var receipt string

		if success {
			for _, item := range callback.CallbackMetadata.Items {
				switch item.Name {
				case "Amount":
					switch value := item.Value.(type) {
					case float64:
						amount = value
					case int:
						amount = float64(value)
					}

				case "PhoneNumber":
					switch value := item.Value.(type) {
					case float64:
						phone = strconv.FormatInt(int64(value), 10)
					case int:
						phone = strconv.Itoa(value)
					case string:
						phone = strings.TrimSpace(value)
					}

				case "MpesaReceiptNumber":
					if value, ok := item.Value.(string); ok {
						receipt = strings.TrimSpace(value)
					}
				}
			}

			// Do not process successful callbacks with incomplete metadata.
			if amount <= 0 || phone == "" || receipt == "" {
				fmt.Println("Daraja callback rejected: incomplete or invalid payment metadata")
				fmt.Fprintln(w, "Callback received")
				return
			}
		}

		fmt.Printf("Daraja callback result code: %d\n", *callback.ResultCode)

		if err := ConfirmSTKContribution(
			db,
			callback.CheckoutRequestID,
			success,
			amount,
			phone,
			receipt,
		); err != nil {
			fmt.Println("Failed to update Daraja contribution status:", err)
			fmt.Fprintln(w, "Callback received")
			return
		}

		fmt.Println("Daraja callback processed successfully")
		fmt.Fprintln(w, "Callback received")
	}
}
