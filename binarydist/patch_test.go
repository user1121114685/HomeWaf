package binarydist

import (
	"bytes"
	"testing"
)

func TestPatch(t *testing.T) {
	old, newer := randomPair()
	var diff, output bytes.Buffer
	if err := Diff(bytes.NewReader(old), bytes.NewReader(newer), &diff); err != nil {
		t.Fatal(err)
	}
	if err := Patch(bytes.NewReader(old), &output, &diff); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), newer) {
		t.Fatal("random patch differs from expected complete file")
	}
}
func TestPatchHk(t *testing.T) {
	var output bytes.Buffer
	if err := Patch(bytes.NewReader(readFixture(t, "sample.old")), &output, bytes.NewReader(readFixture(t, "sample.patch"))); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), readFixture(t, "sample.new")) {
		t.Fatal("independent reference patch produced incorrect file")
	}
}
