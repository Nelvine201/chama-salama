package main

import (
	
	"os"
)

func getDarajaCredentials() (string, string) {
	consumerKey := os.Getenv("DARAJA_CONSUMER_KEY")
	consumerSecret := os.Getenv("DARAJA_CONSUMER_SECRET")
	return consumerKey, consumerSecret
}
