package wafpipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// This adapter covers single HTTP request attacks from three PL1 CRS suites.
// It tests HomeWaf's category and blocking behavior, not complete FTW semantics.
// The payloads are data only: no target servers, scripts or shell commands run.
func TestCRSUpstreamAttackCorpus(t *testing.T) {
	directory := os.Getenv("HOMEWAF_WAF_CORPUS")
	if directory == "" {
		t.Skip("upstream corpus is exercised by the dedicated security workflow")
	}
	var manifest struct {
		Repository string `json:"repository"`
		Commit     string `json:"commit"`
		Files      []struct {
			Category string `json:"category"`
			Name     string `json:"name"`
			SHA256   string `json:"sha256"`
		} `json:"files"`
	}
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Repository != "coreruleset/coreruleset" || len(manifest.Commit) != 40 || len(manifest.Files) != 3 {
		t.Fatal("unexpected upstream corpus manifest")
	}
	provider := &CRSProvider{Root: filepath.Join("..", "cmd", "homewaf", "exedata", "owasp")}
	policy := DefaultPolicy()
	policy.RuleSet.OWASP = ModeBlock
	policy.RuleSet.BudgetMillis = 1000
	snapshot := &Snapshot{Config: PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://upstream.test", Policy: policy}}
	runtime := &Runtime{RulePrepare: provider.Prepare, RuleSteps: func(s *Snapshot) []Step { return []Step{provider.Step(s)} }}
	checked, excluded := 0, 0
	expected := map[string]string{"942100.yaml": "sqli", "941100.yaml": "xss", "930120.yaml": "lfi"}
	for _, file := range manifest.Files {
		before := checked
		if filepath.Base(file.Name) != file.Name || expected[file.Name] != file.Category || file.Category == "" {
			t.Fatal("invalid corpus filename")
		}
		delete(expected, file.Name)
		data, err := os.ReadFile(filepath.Join(directory, file.Name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatalf("corpus digest mismatch: %s", file.Name)
		}
		var suite struct {
			RuleID int `yaml:"rule_id"`
			Tests  []struct {
				ID      int   `yaml:"test_id"`
				Enabled *bool `yaml:"enabled"`
				Stages  []struct {
					Input struct {
						Method  string            `yaml:"method"`
						URI     string            `yaml:"uri"`
						Data    any               `yaml:"data"`
						Headers map[string]string `yaml:"headers"`
						Encoded string            `yaml:"encoded_request"`
						Raw     string            `yaml:"raw_request"`
					} `yaml:"input"`
					Output struct {
						Log struct {
							Expect []int `yaml:"expect_ids"`
						} `yaml:"log"`
					} `yaml:"output"`
				} `yaml:"stages"`
			} `yaml:"tests"`
		}
		if err := yaml.Unmarshal(data, &suite); err != nil {
			t.Fatal(err)
		}
		for _, item := range suite.Tests {
			if (item.Enabled != nil && !*item.Enabled) || len(item.Stages) != 1 {
				excluded++
				continue
			}
			stage := item.Stages[0]
			positive := false
			for _, id := range stage.Output.Log.Expect {
				positive = positive || id == suite.RuleID
			}
			body, textBody := stage.Input.Data.(string)
			if !positive || stage.Input.Encoded != "" || stage.Input.Raw != "" || (stage.Input.Data != nil && !textBody) {
				excluded++
				continue
			}
			t.Run(fmt.Sprintf("%s/%d-%d", file.Category, suite.RuleID, item.ID), func(t *testing.T) {
				if !strings.HasPrefix(stage.Input.URI, "/") || strings.HasPrefix(stage.Input.URI, "//") {
					t.Fatal("corpus must use an origin-form request target")
				}
				request, err := http.NewRequest(stage.Input.Method, "http://example.test"+stage.Input.URI, strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				for key, value := range stage.Input.Headers {
					if strings.EqualFold(key, "Host") {
						request.Host = value
					} else {
						request.Header.Set(key, value)
					}
				}
				if body != "" && request.Header.Get("Content-Type") == "" {
					request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				}
				result, err := runtime.Explain(snapshot, request, testIP())
				if err != nil {
					t.Fatal(err)
				}
				categoryHit := false
				for _, hit := range result.Hits {
					categoryHit = categoryHit || hit.Category == file.Category
				}
				if result.Verdict.Action != Deny || !categoryHit {
					t.Fatalf("upstream attack missed at %s: verdict=%+v hits=%+v", manifest.Commit, result.Verdict, result.Hits)
				}
			})
			checked++
		}
		if checked == before {
			t.Fatalf("no supported attack cases remain for %s", file.Category)
		}
	}
	if checked < 30 {
		t.Fatalf("upstream coverage unexpectedly empty or reduced: %d attacks", checked)
	}
	t.Logf("OWASP CRS %s: checked %d attack requests; %d negative, disabled, multi-stage or raw-wire cases are outside this adapter", manifest.Commit, checked, excluded)
	for _, target := range []string{"/", "/photos?q=family+photo", "/files?name=holiday.jpg", "/search?q=O%27Reilly+books"} {
		request, err := http.NewRequest("GET", "http://example.test"+target, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("User-Agent", "Mozilla/5.0")
		request.Header.Set("Accept", "text/html")
		result, err := runtime.Explain(snapshot, request, testIP())
		if err != nil || result.Verdict.Action != Allow {
			t.Fatalf("benign control blocked: %s %+v %v", target, result, err)
		}
	}
}
