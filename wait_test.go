package usesmileid

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// The API never returns the literal status "complete": a finished job carries
// its decision as the status, so the helper must poll on "processing" and
// return on any decision, whatever it is.
func TestWaitUntilCompleteProcessingThenTerminalDecision(t *testing.T) {
	for _, terminal := range []string{"clear", "block", "attention", "error"} {
		t.Run(terminal, func(t *testing.T) {
			var calls int32
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v3/token" {
					serveToken(w)
					return
				}
				if atomic.AddInt32(&calls, 1) < 3 {
					w.WriteHeader(http.StatusAccepted)
					fmt.Fprint(w, `{"status":"processing","job_id":"job_x","message":"still processing"}`)
					return
				}
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{"status":%q,"job_id":"job_x","user_id":"user_x","message":"Job completed"}`, terminal)
			})

			js, err := c.Verifications.WaitUntilComplete(context.Background(), "job_x", WaitOptions{
				Interval: 5 * time.Millisecond,
				Timeout:  2 * time.Second,
			})
			if err != nil {
				t.Fatalf("WaitUntilComplete: %v", err)
			}
			if js.Status != terminal {
				t.Errorf("status = %q, want %q", js.Status, terminal)
			}
			if !js.IsComplete() {
				t.Errorf("IsComplete() = false for status %q", js.Status)
			}
			if got := atomic.LoadInt32(&calls); got != 3 {
				t.Errorf("polled %d times, want 3 (two processing then terminal)", got)
			}
		})
	}
}

func TestIsCompleteIsFalseWhilePending(t *testing.T) {
	for _, status := range []string{"processing", "not_found", "", "Processing", " NOT_FOUND "} {
		if (JobStatus{Status: status}).IsComplete() {
			t.Errorf("IsComplete() = true for status %q", status)
		}
	}
}

func TestWaitUntilCompleteTimeout(t *testing.T) {
	// The status handler blocks for longer than the whole poll budget, so the
	// poll deadline deterministically expires while a Retrieve is in flight.
	// The resulting context.DeadlineExceeded must surface as TimeoutError,
	// not ConnectionError.
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"status":"processing","job_id":"job_x"}`)
	})

	_, err := c.Verifications.WaitUntilComplete(context.Background(), "job_x", WaitOptions{
		Interval: 5 * time.Millisecond,
		Timeout:  40 * time.Millisecond,
	})
	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("err = %T %v, want *TimeoutError", err, err)
	}
}

func TestWaitUntilCompleteCallerCancellationIsNotTimeout(t *testing.T) {
	// A caller cancelling their own context mid-poll must surface as the
	// caller's error, never as the poll helper's TimeoutError.
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"status":"processing","job_id":"job_x"}`)
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	_, err := c.Verifications.WaitUntilComplete(ctx, "job_x", WaitOptions{
		Interval: 5 * time.Millisecond,
		Timeout:  10 * time.Second,
	})
	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		t.Fatalf("caller cancellation was converted to TimeoutError: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %T %v, want an error wrapping context.Canceled", err, err)
	}
}

func TestWaitUntilCompleteNotFoundTerminal(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"status":"not_found","job_id":"job_x","user_id":"unknown","message":"not found"}`)
	})

	js, err := c.Verifications.WaitUntilComplete(context.Background(), "job_x", WaitOptions{
		Interval:               5 * time.Millisecond,
		Timeout:                2 * time.Second,
		TreatNotFoundAsPending: Bool(false),
	})
	if err != nil {
		t.Fatalf("WaitUntilComplete: %v", err)
	}
	if js.Status != "not_found" {
		t.Errorf("status = %q, want not_found", js.Status)
	}
}

func TestWaitUntilCompleteNotFoundPendingByDefault(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"status":"not_found","job_id":"job_x","user_id":"unknown","message":"not found"}`)
	})

	// Default TreatNotFoundAsPending is true, so a persistently not-found job
	// polls until the timeout.
	_, err := c.Verifications.WaitUntilComplete(context.Background(), "job_x", WaitOptions{
		Interval: 5 * time.Millisecond,
		Timeout:  40 * time.Millisecond,
	})
	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("err = %T, want *TimeoutError", err)
	}
}
