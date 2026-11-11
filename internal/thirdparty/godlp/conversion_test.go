package dlp

import "testing"

func TestConversionsDoNotAliasMutableMemory(t *testing.T) {
	input := []byte("private text")
	s := B2S(input)
	input[0] = 'X'
	if s != "private text" {
		t.Fatal("byte mutation changed an existing string")
	}
	b := S2B(s)
	b[0] = 'Y'
	if s != "private text" {
		t.Fatal("byte conversion mutated string storage")
	}
}
