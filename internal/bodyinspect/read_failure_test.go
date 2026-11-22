package bodyinspect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
	"testing/synctest"
	"time"
)

func TestInspectPrefixPreservesPartialDataAndReadProgress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reader, writer := io.Pipe()
		r := httptest.NewRequest("POST", "/", reader)
		defer r.Body.Close()
		go func() {
			time.Sleep(200 * time.Millisecond)
			_, _ = writer.Write([]byte("abc"))
			time.Sleep(300 * time.Millisecond)
			_, _ = writer.Write([]byte("def"))
			time.Sleep(2 * time.Second)
			_ = writer.CloseWithError(fmt.Errorf("private transport detail: %w", os.ErrDeadlineExceeded))
		}()
		data, f, err := InspectPrefix(r, 32)
		if !errors.Is(err, os.ErrDeadlineExceeded) || string(data) != "abcdef" || f == nil || !f.Valid() ||
			f.Cause != ReadTimeout || f.BytesRead != 6 || f.ElapsedMS != 2500 || *f.IdleMS != 2000 {
			t.Fatalf("unexpected read progress: %+v, %v", f, err)
		}
		replayed, err := io.ReadAll(r.Body)
		if string(replayed) != "abcdef" || !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("original read not preserved: %q, %v", replayed, err)
		}
	})
}

func TestInspectPrefixClassifiesOnlyObservedFailures(t *testing.T) {
	for _, tc := range []struct {
		err      error
		canceled bool
		want     ReadCause
	}{
		{os.ErrDeadlineExceeded, true, ReadTimeout}, {context.Canceled, false, RequestCanceled},
		{io.ErrClosedPipe, true, RequestCanceled}, {io.ErrUnexpectedEOF, true, PrematureEOF},
		{syscall.ECONNRESET, true, ConnectionReset}, {errors.New("private detail"), false, UnknownError},
	} {
		r := httptest.NewRequest("POST", "/", iotest.ErrReader(tc.err))
		ctx, cancel := context.WithCancel(r.Context())
		if tc.canceled {
			cancel()
		}
		_, f, _ := InspectPrefix(r.WithContext(ctx), 10)
		cancel()
		if f == nil || !f.Valid() || f.Cause != tc.want || f.BytesRead != 0 || f.IdleMS != nil {
			t.Fatalf("unexpected failure for %v: %+v", tc.err, f)
		}
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader("abcdefgh"))
	data, f, err := InspectPrefix(r, 3)
	all, replayErr := io.ReadAll(r.Body)
	if err != nil || f != nil || string(data) != "abcd" || replayErr != nil || string(all) != "abcdefgh" {
		t.Fatal("reading a bounded prefix changed the successful upload")
	}
}
