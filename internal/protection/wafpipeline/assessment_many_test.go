package wafpipeline

import (
	"context"
	"fmt"
	"testing"
)

func TestAssessmentManyAggregatesEverySourceAndCancels(t *testing.T) {
	r, s, controls := assessmentFixture()
	input := AssessmentInput{Samples: BuiltinAssessmentSamples(), Pressure: 60, PressureMode: "mixed"}
	for i := 2; i < 82; i++ {
		input.Sources = append(input.Sources, AssessmentSource{IP: fmt.Sprintf("198.51.100.%d", i), ExpectAccess: true})
	}
	input.Sources = append(input.Sources, AssessmentSource{IP: "2001:db8::25", ExpectAccess: true})
	done, total := 0, 0
	result, err := r.AssessMany(context.Background(), s, input, func(d, n int) { done, total = d, n })
	if err != nil {
		t.Fatal(err)
	}
	if result.Evaluations != 81*len(input.Samples) || len(result.SourceResults) != 81 || done != 81 || total != 81 || len(result.Rows) != 128 {
		t.Fatalf("lost results: evaluations=%d sources=%d progress=%d/%d traces=%d", result.Evaluations, len(result.SourceResults), done, total, len(result.Rows))
	}
	one := input
	one.Sources = input.Sources[:1]
	one.Pressure = 0
	expected, err := r.Assess(context.Background(), s, one)
	if err != nil {
		t.Fatal(err)
	}
	if result.Score == nil || expected.Score == nil || *result.Score != *expected.Score {
		t.Fatal("aggregation changed scoring")
	}
	if len(controls.Rates.entries) != 0 || r.Requests.Load() != 0 || s.Compiled != nil {
		t.Fatal("changed live runtime")
	}
	s.Config.Policy.Network.ListenIPVersion = "ipv4"
	reduced, err := r.AssessMany(context.Background(), s, input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reduced.Pressure.Evaluations >= result.Pressure.Evaluations || reduced.Pressure.Lanes[1].Evaluations != 5 {
		t.Fatal("disabled family pressure not reduced")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = r.AssessMany(ctx, s, input, func(int, int) { cancel() })
	if err != context.Canceled {
		t.Fatalf("cancellation lost: %v", err)
	}
	input.Sources = append(input.Sources, AssessmentSource{IP: "::ffff:198.51.100.2"})
	if _, err = r.AssessMany(context.Background(), s, input, nil); err == nil {
		t.Fatal("equivalent duplicate accepted")
	}
}
