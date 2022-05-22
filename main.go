package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	sdk "github.com/PagerDuty/go-pagerduty/webhookv3"
)

const (
	port = 8080

	envCustomHeaderName   = "PAGERDUTY_TEKTON_INTERCEPTOR_CUSTOM_HEADER_NAME"
	envCustomHeaderSecret = "PAGERDUTY_TEKTON_INTERCEPTOR_CUSTOM_HEADER_SECRET" /*
		#nosec -- just the env name, not the value*/
	// the webhook header for this is X-PagerDuty-Signature.
	envWebhookSecretToken = "PAGERDUTY_TEKTON_INTERCEPTOR_WEBHOOK_TOKEN" /*
		#nosec -- just the env name, not the value*/

	webhookBodyReaderLimit = 2 * 1024 * 1024 // 2MB
)

func validatePayload(r *http.Request, secretToken string) (err error) {
	err = sdk.VerifySignature(r, secretToken)

	if err != nil {
		return fmt.Errorf("could not verify the signature: %w", err)
	}
	return nil
}

// WebhookEventDetails contains the data from the pagerduty payload.
type WebhookEventDetails struct {
	Event struct {
		EventType string `json:"event_type"`
		ID        string `json:"id"`
	} `yaml:"event"`
}

func extractEventID(r *http.Request) (WebhookEventDetails, error) {
	var webhookEventDetails WebhookEventDetails

	b, err := io.ReadAll(io.LimitReader(r.Body, webhookBodyReaderLimit))
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

func getBodyBytes(request *http.Request) ([]byte, error) {

	//code snippet
	bodyBytes, err := io.ReadAll(io.LimitReader(request.Body, webhookBodyReaderLimit))

	//---------- optioninal ---------------------
	//handling Errors
	if err != nil {
		return nil, err
	}

	orb := request.Body

	defer func() { _ = orb.Close() }()

	request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return bodyBytes, nil

}

// main function.
func main() {
	customHeaderName := os.Getenv(envCustomHeaderName)
	customHeaderSecret := os.Getenv(envCustomHeaderSecret)
	webhookSecretToken := os.Getenv(envWebhookSecretToken)
	// for explanation on why I chose this query,
	// see https://stackoverflow.com/questions/23025694/is-there-no-xor-operator-for-booleans-in-golang
	isCustomHeaderInvalid := (customHeaderSecret == "") != (customHeaderName == "")

	if webhookSecretToken == "" {
		log.Fatalf("the env '%s' is required, but not set", envWebhookSecretToken)
	}

	if isCustomHeaderInvalid {
		log.Fatalf("if enabled, both envs need to he set, but one is empty: (%s=%s), (%s=%s)",
			envCustomHeaderName, customHeaderName,
			envCustomHeaderSecret, customHeaderSecret,
		)
	}

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {

		var err error

		fmt.Printf("%v: got request\n", time.Now().UTC())
		// if the custom header feature is enabled.
		if (customHeaderSecret != "") && (webhookSecretToken != "") {
			h := request.Header.Get(customHeaderName)
			if h != customHeaderSecret {
				strErr := fmt.Sprintf("the header '%s' is not matching the secret value", customHeaderName)
				http.Error(writer, strErr, http.StatusBadRequest)

				return
			}
		}

		err = validatePayload(request, webhookSecretToken)
		if err != nil {
			http.Error(writer, fmt.Sprint(err), http.StatusBadRequest)

			return
		}

		bodyBytes, err := getBodyBytes(request)

		if err != nil {
			http.Error(writer, fmt.Sprint(err), http.StatusBadRequest)
			return
		}
		id, err := extractEventID(request)
		if err != nil {
			http.Error(writer, fmt.Sprint(err), http.StatusBadRequest)

			return
		}
		n, err := writer.Write(bodyBytes)
		if err != nil {
			log.Printf("Failed to write response for pagerduty "+
				"event ID: '%s' "+
				"Bytes writted: '%d' "+
				"Error: '%q'\n", id.Event.ID, n, err)
		}
		fmt.Printf("%v: request is valid\n", time.Now().UTC())
	})
	fmt.Printf("connecting on port: %d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil))
}
