package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	sdk "github.com/PagerDuty/go-pagerduty/webhookv3"
)

const (
	port = 8080

	envCustomHeaderName   = "PAGERDUTY_TEKTON_INTERCEPTOR_CUSTOM_HEADER_NAME"
	envCustomHeaderSecret = "PAGERDUTY_TEKTON_INTERCEPTOR_CUSTOM_HEADER_SECRET"
	envWebhookSecretToken = "PAGERDUTY_TEKTON_INTERCEPTOR_WEBHOOK_TOKEN"

	webhookBodyReaderLimit = 2 * 1024 * 1024 // 2MB
)

func ValidatePayload(r *http.Request, secretToken string) (payload []byte, err error) {
	err = sdk.VerifySignature(r, secretToken)
	if err != nil {
		return nil, fmt.Errorf("could not verify the signature: %w", err)
	}

	orb := r.Body

	b, err := ioutil.ReadAll(io.LimitReader(r.Body, webhookBodyReaderLimit))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	defer func() { _ = orb.Close() }()

	return b, nil
}

type WebhookEventDetails struct {
	Event struct {
		EventType  string    `yaml:"event_type"`
		ID         string    `yaml:"id"`
		OccurredAt time.Time `yaml:"occurred_at"`
	} `yaml:"event"`
}

func ExtractEventID(r *http.Request) (WebhookEventDetails, error) {
	var webhookEventDetails WebhookEventDetails
	b, err := ioutil.ReadAll(io.LimitReader(r.Body, webhookBodyReaderLimit))
	if err != nil {
		return webhookEventDetails, fmt.Errorf("failed to read response body: %w", err)
	}

	orb := r.Body

	defer func() { _ = orb.Close() }()

	err = json.Unmarshal(b, &webhookEventDetails)
	if err != nil {
		return webhookEventDetails, fmt.Errorf("could not marshal into WebhookEventDetails: %w", err)
	}

	return webhookEventDetails, nil
}

// main function
func main() {
	customHeaderName := os.Getenv(envCustomHeaderName)
	customHeaderSecret := os.Getenv(envCustomHeaderSecret)
	webhookSecretToken := os.Getenv(envWebhookSecretToken)
	isAnyEnvEmpty := customHeaderName == "" ||
		customHeaderSecret == "" ||
		webhookSecretToken == ""

	if isAnyEnvEmpty {
		log.Fatalf("one of the required env vars is empty: (%s=%s), (%s=%s), (%s=%s)",
			envCustomHeaderName, customHeaderName,
			envCustomHeaderSecret, customHeaderSecret,
			envWebhookSecretToken, webhookSecretToken)
	}

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		payload, err := ValidatePayload(request, webhookSecretToken)
		if err != nil {
			http.Error(writer, fmt.Sprint(err), http.StatusBadRequest)
		}
		id, err := ExtractEventID(request)
		if err != nil {
			http.Error(writer, fmt.Sprint(err), http.StatusBadRequest)
		}
		n, err := writer.Write(payload)
		if err != nil {
			log.Printf("Failed to write response for gitea event ID: %s. Bytes writted: %d. Error: %q", id.Event.ID, n, err)
		}
	})
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
