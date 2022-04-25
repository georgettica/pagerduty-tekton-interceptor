package main

import (
	"context"
	"io"
	"net/http"
	"strings"

	triggersv1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	"github.com/tektoncd/triggers/pkg/interceptors"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	corev1lister "k8s.io/client-go/listers/core/v1"
)

var _ triggersv1.InterceptorInterface = (*Interceptor)(nil)

type Interceptor struct {
	SecretLister corev1lister.SecretLister
	Logger       *zap.SugaredLogger
}

func NewInterceptor(sl corev1lister.SecretLister, l *zap.SugaredLogger) *Interceptor {
	return &Interceptor{
		SecretLister: sl,
		Logger:       l,
	}
}

// PagerDutyInterceptor provides a webhook to intercept and pre-process events.
type PagerDutyInterceptor struct {
	SecretRef *triggersv1.SecretRef `json:"secretRef,omitempty"`
	// +listType=atomic
	EventTypes []string `json:"eventTypes,omitempty"`
}

func (w *Interceptor) Process(ctx context.Context, r *triggersv1.InterceptorRequest) *triggersv1.InterceptorResponse {
	p := PagerDutyInterceptor{}
	if err := interceptors.UnmarshalParams(r.InterceptorParams, &p); err != nil {
		return interceptors.Failf(codes.InvalidArgument, "failed to parse interceptor params: %v", err)
	}

	headers := interceptors.Canonical(r.Header)

	// Check if the event type is in the allow-list
	if p.EventTypes != nil {
		actualEvent := http.Header(r.Header).Get("X-Event-Key")
		isAllowed := false

		for _, allowedEvent := range p.EventTypes {
			if actualEvent == allowedEvent {
				isAllowed = true

				break
			}
		}

		if !isAllowed {
			return interceptors.Failf(codes.FailedPrecondition, "event type %s is not allowed", actualEvent)
		}
	}

	// Next validate secrets if set
	if p.SecretRef != nil {
		// Check the secret to see if it is empty
		if p.SecretRef.SecretKey == "" {
			return interceptors.Fail(codes.FailedPrecondition, "pagerduty interceptor secretRef.secretKey is empty")
		}

		header := headers.Get("X-Hub-Signature")
		if header == "" {
			return interceptors.Fail(codes.InvalidArgument, "no X-Hub-Signature header set")
		}

		ns, _ := triggersv1.ParseTriggerID(r.Context.TriggerID)

		secretToken, err := interceptors.GetSecretToken(nil, w.SecretLister, p.SecretRef, ns)
		if err != nil {
			return interceptors.Failf(codes.FailedPrecondition, "error getting secret: %v", err)
		}

		req := new(http.Request)
		responseBody := io.NopCloser(strings.NewReader(r.Body))
		req.Header = headers
		req.Body = responseBody

		if err := sdk.VerifySignature(req, secretToken); err != nil {
			return interceptors.Failf(codes.FailedPrecondition, err.Error())
		}
	}

	return &triggersv1.InterceptorResponse{
		Continue: true,
	}
}

func main() {}
