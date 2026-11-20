package utils

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPRetryError retains an upstream's retry deadline without retaining its
// URL, response body or credentials.
type HTTPRetryError struct {
	Status    int
	NotBefore time.Time
	Operation string
}

func (e *HTTPRetryError) Error() string {
	return fmt.Sprintf("%s返回 HTTP %d，最早重试时间 %s", e.Operation, e.Status, e.NotBefore.Format(time.RFC3339))
}
func (e *HTTPRetryError) RetryAt() time.Time { return e.NotBefore }
func NewHTTPRetryError(status int, value, operation string, now time.Time) *HTTPRetryError {
	next := now.Add(time.Minute)
	if status == http.StatusTooManyRequests {
		next = now.Add(6 * time.Hour)
	}
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && seconds >= 0 && seconds <= 31536000 {
		next = now.Add(time.Duration(seconds) * time.Second)
	} else if parsed, err := http.ParseTime(value); err == nil {
		next = parsed
	}
	if next.Before(now.Add(time.Second)) {
		next = now.Add(time.Second)
	}
	return &HTTPRetryError{Status: status, NotBefore: next, Operation: operation}
}
