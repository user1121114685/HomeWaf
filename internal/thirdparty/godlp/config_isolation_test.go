package dlp

import (
	"HomeWaf/internal/thirdparty/godlp/conf"
	"fmt"
	"go.yaml.in/yaml/v4"
	"io"
	"os"
	"strings"
	"testing"
)

func TestIndependentEnginesDoNotRewritePackageDefaults(t *testing.T) {
	a, err := NewEngine("port-a")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.ApplyConfigDefault(); err != nil {
		t.Fatal(err)
	}
	beforeInput, beforeRules := a.(*Engine).maxLogInput, a.(*Engine).maxRegexRuleID
	config, err := conf.NewDlpConf(DEF_CFG)
	if err != nil {
		t.Fatal(err)
	}
	config.Global.MaxLogInput = 17
	config.Global.MaxRegexRuleID = 345
	raw, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		b, e := NewEngine("port-b")
		if e == nil {
			e = b.ApplyConfig(string(raw))
		}
		if e == nil && (b.(*Engine).maxLogInput != 17 || b.(*Engine).maxRegexRuleID != 345) {
			e = fmt.Errorf("instance limits lost")
		}
		done <- e
	}()
	_, _, _ = a.Deidentify("email=family@example.com")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if a.(*Engine).maxLogInput != beforeInput || a.(*Engine).maxRegexRuleID != beforeRules || DEF_MAX_LOG_INPUT != 1024 || DEF_MAX_REGEX_RULE_ID != 0 {
		t.Fatal("configuration changed another engine or package defaults")
	}
}

func TestRecoveryDoesNotPrintSensitivePanicOrStack(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	previous := os.Stderr
	os.Stderr = writer
	func() {
		defer func() { os.Stderr = previous; _ = writer.Close() }()
		func() { defer (&Engine{}).recoveryImpl(); panic("private-response-value") }()
	}()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "DLP processing failed") || strings.Contains(string(output), "private-response-value") || strings.Contains(string(output), ".go:") {
		t.Fatal("DLP failure log leaked input or stack")
	}
}
