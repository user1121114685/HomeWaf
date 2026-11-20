package email

import (
	"mime"
	"net/mail"
	"strings"
	"testing"
)

func TestEmailHeaderAndBodyBoundaries(t *testing.T) {
	n := EmailNotifier{FromEmail: "waf@example.test", FromName: "家庭", ToEmails: []string{"ops@example.test"}}
	body := n.markdownToHTML("**Service** <family>\nstatus: A & B")
	raw, err := n.message("家庭通知", body)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	title, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || title != "家庭通知" || !strings.Contains(body, "&lt;family&gt;") || !strings.Contains(body, "A &amp; B") {
		t.Fatalf("unexpected message: %v %q %s", err, title, body)
	}
	for _, change := range []func(*EmailNotifier, *string){
		func(_ *EmailNotifier, s *string) { *s = "two\nlines" },
		func(n *EmailNotifier, _ *string) { n.FromName = "two\rlines" },
		func(n *EmailNotifier, _ *string) { n.FromEmail = "invalid\naddress" },
		func(n *EmailNotifier, _ *string) { n.SetRecipients([]string{"two\nlines"}) },
	} {
		tmp, subject := n, "notice"
		change(&tmp, &subject)
		if _, err := tmp.message(subject, body); err == nil {
			t.Fatal("invalid header accepted")
		}
	}
}
