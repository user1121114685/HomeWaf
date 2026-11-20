package innerbean

import "HomeWaf/internal/logsanitize"

// RedactedCopy is the common immutable value used by database, file and Kafka
// sinks. Detection still uses the original request-scoped WebLog.
func (w *WebLog) RedactedCopy() *WebLog {
	if w == nil {
		return nil
	}
	out := *w
	out.HEADER = logsanitize.Headers(w.HEADER)
	out.ResHeader = logsanitize.Headers(w.ResHeader)
	out.COOKIES = logsanitize.Cookies(w.COOKIES)
	out.URL = logsanitize.URI(w.URL)
	out.RawQuery = logsanitize.Query(w.RawQuery)
	out.REFERER = logsanitize.URI(w.REFERER)
	if w.SrcURL != nil {
		out.SrcURL = []byte(logsanitize.URI(string(w.SrcURL)))
	}
	out.POST_FORM = logsanitize.Query(w.POST_FORM)
	out.BODY = logsanitize.Body(w.BODY)
	out.RES_BODY = logsanitize.Body(w.RES_BODY)
	out.SrcByteBody = nil
	out.SrcByteResBody = nil
	out.BodyDecoded = ""
	out.CookiesDecoded = ""
	out.BodyValues = nil
	out.BodyFields = nil
	out.BodyHash = "" // Unsalted hashes of low-entropy credentials are unsafe.
	out.USER_AGENT = logsanitize.Cut(w.USER_AGENT)
	out.RULE = logsanitize.Cut(w.RULE)
	return &out
}

func (w *WebLog) SafeForQueue() any { return w.RedactedCopy() }

func (w *WebLog) QueuePriority() int {
	if w != nil && w.IsSecurityEvent() {
		return 2
	}
	return 0
}
func (w *WebLog) QueueKey() string {
	if w == nil {
		return "nil"
	}
	return w.HOST_CODE
}
func (w *WebLog) QueueSize() int64 {
	if w == nil {
		return 1
	}
	return int64(1024 + len(w.HEADER) + len(w.ResHeader) + len(w.COOKIES) + len(w.URL) + len(w.RawQuery) + len(w.REFERER) + len(w.SrcURL) + len(w.POST_FORM) + len(w.BODY) + len(w.RES_BODY) + len(w.USER_AGENT) + len(w.RULE))
}
