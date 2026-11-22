package bodyinspect

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"runtime"
	"syscall"
	"time"
)

type ReadCause string

const (
	ReadTimeout     ReadCause = "timeout"
	RequestCanceled ReadCause = "request_canceled"
	PrematureEOF    ReadCause = "unexpected_eof"
	ConnectionReset ReadCause = "connection_reset"
	UnknownError    ReadCause = "unknown"
)

type ReadFailure struct {
	Cause     ReadCause `json:"cause"`
	BytesRead int       `json:"bytes_read"`
	ElapsedMS int64     `json:"elapsed_ms"`
	// Nil means no bytes arrived during inspection.
	IdleMS *int64 `json:"idle_ms,omitempty"`
}

func (f ReadFailure) Valid() bool {
	switch f.Cause {
	case ReadTimeout, RequestCanceled, PrematureEOF, ConnectionReset, UnknownError:
	default:
		return false
	}
	return f.BytesRead >= 0 && f.BytesRead <= maxPrefixBytes+1 && f.ElapsedMS >= 0 &&
		((f.BytesRead == 0 && f.IdleMS == nil) || (f.BytesRead > 0 && f.IdleMS != nil && *f.IdleMS >= 0 && *f.IdleMS <= f.ElapsedMS))
}

// InspectPrefix measures only the inspection read. The restored body retains
// the original error and unread tail, without tracking the subsequent upload.
func InspectPrefix(r *http.Request, limit int64) ([]byte, *ReadFailure, error) {
	started := time.Now()
	var lastRead time.Time
	data, err := readPrefix(r, limit, &lastRead)
	if err == nil {
		return data, nil, nil
	}
	finished := time.Now()
	failure := &ReadFailure{Cause: readCause(err, r.Context().Err()), BytesRead: len(data), ElapsedMS: finished.Sub(started).Milliseconds()}
	if !lastRead.IsZero() {
		idle := finished.Sub(lastRead).Milliseconds()
		failure.IdleMS = &idle
	}
	return data, failure, err
}

func readCause(err, ctxErr error) ReadCause {
	const winsockConnectionReset = syscall.Errno(10054) // WSAECONNRESET on Windows
	var transportError net.Error
	switch {
	case errors.As(err, &transportError) && transportError.Timeout():
		return ReadTimeout
	case errors.Is(err, io.ErrUnexpectedEOF):
		return PrematureEOF
	case errors.Is(err, syscall.ECONNRESET), runtime.GOOS == "windows" && errors.Is(err, winsockConnectionReset):
		return ConnectionReset
	case errors.Is(err, context.Canceled), errors.Is(ctxErr, context.Canceled):
		return RequestCanceled
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return ReadTimeout
	default:
		return UnknownError
	}
}
