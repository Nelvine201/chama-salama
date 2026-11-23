package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDarajaCallbackHandler(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		wantStatus  string
		wantReceipt string
	}{
		{
			name: "successful payment callback",
			payload: `{
				"Body": {
					"stkCallback": {
						"CheckoutRequestID": "ws_CO_test123",
						"ResultCode": 0,
						"CallbackMetadata": {
							"Item": [
								{"Name": "Amount", "Value": 100},
								{"Name": "MpesaReceiptNumber", "Value": "QGH123ABC"},
								{"Name": "PhoneNumber", "Value": 254712345678}
							]
						}
					}
				}
			}`,
			wantStatus:  "synced",
			wantReceipt: "QGH123ABC",
		},
		{
			name: "expired payment callback",
			payload: `{
				"Body": {
					"stkCallback": {
						"CheckoutRequestID": "ws_CO_test123",
						"ResultCode": 1037
					}
				}
			}`,
			wantStatus: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContributionDB(t)
			insertPendingContribution(t, db)

			req := httptest.NewRequest(
				http.MethodPost,
				"/callback",
				strings.NewReader(tt.payload),
			)
			rec := httptest.NewRecorder()

			handler := darajaCallbackHandler(db)
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected HTTP %d, got %d", http.StatusOK, rec.Code)
			}

			var gotStatus string
			err := db.QueryRow(`
				SELECT status
				FROM contributions
				WHERE checkout_request_id = ?
			`, "ws_CO_test123").Scan(&gotStatus)
			if err != nil {
				t.Fatal(err)
			}

			if gotStatus != tt.wantStatus {
				t.Fatalf("expected status %q, got %q", tt.wantStatus, gotStatus)
			}

			if tt.wantReceipt != "" {
				var gotReceipt string
				err := db.QueryRow(`
					SELECT mpesa_receipt_number
					FROM contributions
					WHERE checkout_request_id = ?
				`, "ws_CO_test123").Scan(&gotReceipt)
				if err != nil {
					t.Fatal(err)
				}

				if gotReceipt != tt.wantReceipt {
					t.Fatalf("expected receipt %q, got %q", tt.wantReceipt, gotReceipt)
				}
			}
		})
	}
}

func TestDarajaCallbackRejectsNonPost(t *testing.T) {
	db := setupContributionDB(t)

	req := httptest.NewRequest(
		http.MethodGet,
		"/callback",
		nil,
	)
	rec := httptest.NewRecorder()

	handler := darajaCallbackHandler(db)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected HTTP %d, got %d",
			http.StatusMethodNotAllowed,
			rec.Code,
		)
	}
}

func TestDarajaCallbackRejectsInvalidPayloads(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "malformed JSON",
			payload: `{"Body":`,
		},
		{
			name: "missing checkout request ID",
			payload: `{
				"Body": {
					"stkCallback": {
						"ResultCode": 0
					}
				}
			}`,
		},
		{
			name: "missing result code",
			payload: `{
				"Body": {
					"stkCallback": {
						"CheckoutRequestID": "ws_CO_test123"
					}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContributionDB(t)
			insertPendingContribution(t, db)

			req := httptest.NewRequest(
				http.MethodPost,
				"/callback",
				strings.NewReader(tt.payload),
			)
			rec := httptest.NewRecorder()

			handler := darajaCallbackHandler(db)
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected HTTP %d, got %d",
					http.StatusBadRequest,
					rec.Code,
				)
			}

			var status string
			err := db.QueryRow(`
				SELECT status
				FROM contributions
				WHERE checkout_request_id = ?
			`, "ws_CO_test123").Scan(&status)
			if err != nil {
				t.Fatal(err)
			}

			if status != "pending" {
				t.Errorf("status = %q, want pending", status)
			}
		})
	}
}

func TestDarajaCallbackRejectsIncompleteSuccessMetadata(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name: "missing amount",
			payload: `{
				"Body": {
					"stkCallback": {
						"CheckoutRequestID": "ws_CO_test123",
						"ResultCode": 0,
						"CallbackMetadata": {
							"Item": [
								{"Name": "MpesaReceiptNumber", "Value": "QGH123ABC"},
								{"Name": "PhoneNumber", "Value": 254712345678}
							]
						}
					}
				}
			}`,
		},
		{
			name: "missing receipt",
			payload: `{
				"Body": {
					"stkCallback": {
						"CheckoutRequestID": "ws_CO_test123",
						"ResultCode": 0,
						"CallbackMetadata": {
							"Item": [
								{"Name": "Amount", "Value": 100},
								{"Name": "PhoneNumber", "Value": 254712345678}
							]
						}
					}
				}
			}`,
		},
		{
			name: "missing phone number",
			payload: `{
				"Body": {
					"stkCallback": {
						"CheckoutRequestID": "ws_CO_test123",
						"ResultCode": 0,
						"CallbackMetadata": {
							"Item": [
								{"Name": "Amount", "Value": 100},
								{"Name": "MpesaReceiptNumber", "Value": "QGH123ABC"}
							]
						}
					}
				}
			}`,
		},
		{
			name: "receipt value is null",
			payload: `{
        "Body": {
            "stkCallback": {
                "CheckoutRequestID": "ws_CO_test123",
                "ResultCode": 0,
                "CallbackMetadata": {
                    "Item": [
                        {"Name": "Amount", "Value": 100},
                        {"Name": "MpesaReceiptNumber", "Value": null},
                        {"Name": "PhoneNumber", "Value": 254712345678}
                    ]
                }
            }
        }
    }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupContributionDB(t)
			insertPendingContribution(t, db)

			req := httptest.NewRequest(
				http.MethodPost,
				"/callback",
				strings.NewReader(tt.payload),
			)
			rec := httptest.NewRecorder()

			darajaCallbackHandler(db).ServeHTTP(rec, req)

			// The callback is acknowledged, but invalid metadata
			// must not update the contribution.
			if rec.Code != http.StatusOK {
				t.Fatalf("expected HTTP 200, got %d", rec.Code)
			}

			var status string
			err := db.QueryRow(`
				SELECT status
				FROM contributions
				WHERE checkout_request_id = ?
			`, "ws_CO_test123").Scan(&status)
			if err != nil {
				t.Fatal(err)
			}

			if status != "pending" {
				t.Errorf("status = %q, want pending", status)
			}
		})
	}
}
