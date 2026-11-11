package binarydist

import (
	"bytes"
	"compress/bzip2"
	"encoding/binary"
	"io"
	"math/rand"
	"os"
	"testing"
)

// Compare the three uncompressed BSDIFF blocks exactly. Different bzip2
// encoders can produce different compressed bytes for the same patch program.
func patchBlocks(t *testing.T, patch []byte) [3][]byte {
	t.Helper()
	if len(patch) < 32 || string(patch[:8]) != "BSDIFF40" {
		t.Fatal("invalid patch header")
	}
	x, y := int64(binary.LittleEndian.Uint64(patch[8:16])), int64(binary.LittleEndian.Uint64(patch[16:24]))
	if x < 0 || y < 0 || x > int64(len(patch)-32) || y > int64(len(patch)-32)-x {
		t.Fatal("invalid patch block sizes")
	}
	pieces := [][]byte{patch[32 : 32+x], patch[32+x : 32+x+y], patch[32+x+y:]}
	var out [3][]byte
	for i, p := range pieces {
		b, err := io.ReadAll(bzip2.NewReader(bytes.NewReader(p)))
		if err != nil {
			t.Fatal(err)
		}
		out[i] = b
	}
	return out
}
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func randomPair() ([]byte, []byte) {
	a, b := make([]byte, 1000), make([]byte, 1000)
	_, _ = rand.New(rand.NewSource(2)).Read(b)
	// Disjoint alphabets make the independent all-extra reference exact.
	for i := range b {
		b[i] |= 0x80
	}
	return a, b
}
func TestDiff(t *testing.T) {
	old, newer := randomPair()
	// Independent reference for unrelated inputs: one all-extra control tuple.
	control := make([]byte, 24)
	binary.LittleEndian.PutUint64(control[8:16], uint64(len(newer)))
	cases := []struct {
		name       string
		old, newer []byte
		blocks     [3][]byte
	}{
		{"random", old, newer, [3][]byte{control, {}, newer}},
		{"reference", readFixture(t, "sample.old"), readFixture(t, "sample.new"), patchBlocks(t, readFixture(t, "sample.patch"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var patch bytes.Buffer
			if err := Diff(bytes.NewReader(tc.old), bytes.NewReader(tc.newer), &patch); err != nil {
				t.Fatal(err)
			}
			got := patchBlocks(t, patch.Bytes())
			for i := range got {
				if !bytes.Equal(got[i], tc.blocks[i]) {
					t.Fatalf("reference patch block %d differs: got %d bytes want %d", i, len(got[i]), len(tc.blocks[i]))
				}
			}
			var output bytes.Buffer
			if err := Patch(bytes.NewReader(tc.old), &output, bytes.NewReader(patch.Bytes())); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(output.Bytes(), tc.newer) {
				t.Fatal("patch did not reproduce complete new file")
			}
		})
	}
}
