package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	darajaCallbackMaxAttempts = 10
	darajaCallbackRetryDelay  = 15 * time.Second
)

type darajaCallbackData struct {
	CheckoutRequestID string
	Success           bool
	Amount            float64
	Phone             string
	Receipt           string
	MetadataValid     bool
}

func decodeDarajaCallback(body []byte) (darajaCallbackData, error) {
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
		return darajaCallbackData{}, fmt.Errorf("invalid callback JSON: %w", err)
	}

	callback := payload.Body.StkCallback

	if strings.TrimSpace(callback.CheckoutRequestID) == "" || callback.ResultCode == nil {
		return darajaCallbackData{}, fmt.Errorf("missing callback fields")
	}

	data := darajaCallbackData{
		CheckoutRequestID: strings.TrimSpace(callback.CheckoutRequestID),
		Success:           *callback.ResultCode == 0,
		MetadataValid:     true,
	}

	// Failed or cancelled payments do not require success metadata.
	if !data.Success {
		return data, nil
	}

	for _, item := range callback.CallbackMetadata.Items {
		switch item.Name {
		case "Amount":
			switch value := item.Value.(type) {
			case float64:
				data.Amount = value
			case int:
				data.Amount = float64(value)
			}

		case "PhoneNumber":
			switch value := item.Value.(type) {
			case float64:
				if value > 0 && value == float64(int64(value)) {
					data.Phone = strconv.FormatInt(int64(value), 10)
				}
			case int:
				data.Phone = strconv.Itoa(value)
			case string:
				data.Phone = strings.TrimSpace(value)
			}

		case "MpesaReceiptNumber":
			if value, ok := item.Value.(string); ok {
				data.Receipt = strings.TrimSpace(value)
			}
		}
	}

	if data.Amount <= 0 || data.Phone == "" || data.Receipt == "" {
		data.MetadataValid = false
		return data, nil
	}

	if _, valid := normalizeKenyanPhone(data.Phone); !valid {
		data.MetadataValid = false
	}

	return data, nil
}

func processDarajaCallbackPayload(db *sql.DB, body []byte) error {
	data, err := decodeDarajaCallback(body)
	if err != nil {
		return err
	}

	if !data.MetadataValid {
		return fmt.Errorf("incomplete or invalid payment metadata")
	}

	return ConfirmSTKContribution(
		db,
		data.CheckoutRequestID,
		data.Success,
		data.Amount,
		data.Phone,
		data.Receipt,
	)
}

func darajaCallbackHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		defer r.Body.Close()

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "Invalid callback body", http.StatusBadRequest)
			return
		}

		fmt.Printf("Daraja callback received: body size=%d bytes\n", len(body))

		data, err := decodeDarajaCallback(body)
		if err != nil {
			fmt.Println("Daraja callback rejected:", err)
			http.Error(w, "Invalid callback payload", http.StatusBadRequest)
			return
		}

		if !data.MetadataValid {
			fmt.Println("Daraja callback rejected: incomplete or invalid payment metadata")
			fmt.Fprintln(w, "Callback received")
			return
		}

		_, err = db.Exec(`
			INSERT INTO daraja_callback_queue
				(checkout_request_id, payload, status)
			VALUES (?, ?, 'pending')
			ON CONFLICT(checkout_request_id) DO NOTHING
		`, data.CheckoutRequestID, string(body))
		if err != nil {
			fmt.Println("Failed to save Daraja callback:", err)
			http.Error(w, "Callback could not be saved", http.StatusInternalServerError)
			return
		}

		// Attempt immediate processing after the callback is safely stored.
		// Any processing failure remains available for background retries.
		if _, err := ProcessDarajaCallbackQueue(db); err != nil {
			fmt.Println("Daraja callback immediate processing error:", err)
		}

		fmt.Fprintln(w, "Callback received")
	}
}

func ProcessDarajaCallbackQueue(db *sql.DB) (int, error) {
	rows, err := db.Query(`
		SELECT id, payload
		FROM daraja_callback_queue
		WHERE status = 'pending'
		AND attempts < ?
		ORDER BY id
	`, darajaCallbackMaxAttempts)
	if err != nil {
		return 0, err
	}

	type queueItem struct {
		ID      int64
		Payload string
	}

	var items []queueItem

	for rows.Next() {
		var item queueItem
		if err := rows.Scan(&item.ID, &item.Payload); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}

	if err := rows.Close(); err != nil {
		return 0, err
	}

	processed := 0

	for _, item := range items {
		result, err := db.Exec(`
			UPDATE daraja_callback_queue
			SET attempts = attempts + 1
			WHERE id = ?
			AND status = 'pending'
			AND attempts < ?
		`, item.ID, darajaCallbackMaxAttempts)
		if err != nil {
			return processed, err
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return processed, err
		}

		if affected == 0 {
			continue
		}

		err = processDarajaCallbackPayload(db, []byte(item.Payload))

		if err == nil {
			_, err = db.Exec(`
				UPDATE daraja_callback_queue
				SET status = 'processed',
				    processed_at = CURRENT_TIMESTAMP,
				    last_error = NULL
				WHERE id = ?
			`, item.ID)
			if err != nil {
				fmt.Println("Failed to mark Daraja callback processed:", err)
				continue
			}

			processed++
			continue
		}

		fmt.Printf("Daraja callback processing failed (queue ID %d): %v\n", item.ID, err)

		_, updateErr := db.Exec(`
			UPDATE daraja_callback_queue
			SET last_error = ?,
			    status = CASE
			        WHEN attempts >= ? THEN 'failed'
			        ELSE 'pending'
			    END
			WHERE id = ?
		`, err.Error(), darajaCallbackMaxAttempts, item.ID)

		if updateErr != nil {
			return processed, updateErr
		}
	}

	return processed, nil
}

func startDarajaCallbackWorker(db *sql.DB) {
	go func() {
		process := func() {
			count, err := ProcessDarajaCallbackQueue(db)
			if err != nil {
				fmt.Println("Daraja callback worker error:", err)
				return
			}

			if count > 0 {
				fmt.Printf("Daraja callback worker processed %d callback(s)\n", count)
			}
		}

		process()

		ticker := time.NewTicker(darajaCallbackRetryDelay)
		defer ticker.Stop()

		for range ticker.C {
			process()
		}
	}()
}
