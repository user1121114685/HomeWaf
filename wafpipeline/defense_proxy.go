package wafpipeline

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

var errDefenseResponseHandled = errors.New("defense response handled")

type replayResponseBody struct {
	io.Reader
	io.Closer
}

// Recognize configured business authentication before emitting headers/body.
// Only the configured login path is sampled, with the existing byte limit and
// a hard read deadline; ordinary downloads and streams are never buffered here.
func (f *Forwarder) inspectDefenseLogin(state *forwardRequest, response *http.Response) error {
	s, c, record := state.snapshot, state.request, state.response
	d := s.Config.Policy.Defense
	if !d.Enabled || record.login == nil || record.status >= 500 || record.status == 101 || strings.Contains(strings.ToLower(record.contentType), "text/event-stream") {
		return nil
	}
	if record.login.SuccessBodyContains != "" {
		body := response.Body
		if record.sample == nil {
			record.sample = &responsePrefix{ReadCloser: body, limit: record.login.BodyPrefixBytes}
			body = record.sample
		}
		timer := time.AfterFunc(5*time.Second, func() { _ = body.Close() })
		prefix, err := io.ReadAll(io.LimitReader(body, int64(record.login.BodyPrefixBytes)+1))
		timer.Stop()
		if err != nil {
			record.transportError = true
			_ = body.Close()
			return err
		}
		response.Body = &replayResponseBody{Reader: io.MultiReader(bytes.NewReader(prefix), body), Closer: body}
	}
	f.observe(s, c, nil, record)
	record.loginObserved = true
	v := Verdict{Action: Deny, Stage: 5, Status: record.status, Reason: "business:authentication_failed"}
	if c.RejectResponse != nil && DefenseFailure(c, v) {
		defer func() {
			if handled, _ := c.Values["defense_response"].(bool); handled {
				_ = response.Body.Close()
			}
		}()
		writer, _ := c.Values["response_writer"].(http.ResponseWriter)
		if writer != nil && c.RejectResponse(writer, v) {
			return errDefenseResponseHandled
		}
	}
	return nil
}
