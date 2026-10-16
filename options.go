package usesmileid

import (
	"context"
	"time"
)

// RequestOption customizes a single request. Pass options as the final,
// variadic argument to any resource method.
type RequestOption func(*requestOptions)

type requestOptions struct {
	timeout       *time.Duration
	callbackURL   *string
	enrolledImage bool
}

// WithTimeout overrides the client's per-request timeout for this call.
func WithTimeout(d time.Duration) RequestOption {
	return func(ro *requestOptions) { ro.timeout = &d }
}

// WithCallbackURL sets the callback URL for this call, overriding both the
// value in params and the client's default callback URL.
func WithCallbackURL(u string) RequestOption {
	return func(ro *requestOptions) { ro.callbackURL = &u }
}

// WithEnrolledImage verifies a biometric KYC job against the enrolled selfie
// of its UserID, which is then required. The grant is bound into that job's
// token and the selfie and liveness images may be omitted. Other methods
// ignore it.
func WithEnrolledImage() RequestOption {
	return func(ro *requestOptions) { ro.enrolledImage = true }
}

func resolveOptions(opts []RequestOption) *requestOptions {
	ro := &requestOptions{}
	for _, o := range opts {
		o(ro)
	}
	return ro
}

func (c *Client) withTimeout(ctx context.Context, ro *requestOptions) (context.Context, context.CancelFunc) {
	d := c.cfg.timeout
	if ro.timeout != nil {
		d = *ro.timeout
	}
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

// resolveCallback applies, in order of precedence, a per-request override, the
// value already in params, then the client default. The resolved value must be
// an absolute https URL; anything else returns a *ValidationError before any
// request is made.
func (c *Client) resolveCallback(explicit *string, ro *requestOptions) (*string, error) {
	var cb *string
	switch {
	case ro.callbackURL != nil:
		cb = ro.callbackURL
	case explicit != nil:
		cb = explicit
	case c.cfg.defaultCallbackURL != "":
		d := c.cfg.defaultCallbackURL
		cb = &d
	}
	if cb != nil {
		if err := validateHTTPSURL("callback_url", *cb); err != nil {
			return nil, err
		}
	}
	return cb, nil
}

type enrolledImageKey struct{}

func withEnrolledImageGrant(ctx context.Context) context.Context {
	return context.WithValue(ctx, enrolledImageKey{}, true)
}

func enrolledImageGranted(ctx context.Context) bool {
	granted, _ := ctx.Value(enrolledImageKey{}).(bool)
	return granted
}
