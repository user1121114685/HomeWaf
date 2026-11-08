package capserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// Keep both released browser wire formats interoperable, including one-use tokens.
func TestHomeWafCAPWireFormats(t *testing.T) {
	for _, modern := range []bool{true, false} {
		t.Run(strconv.FormatBool(modern), func(t *testing.T) {
			server := New(&CapConfig{})
			challenge, err := server.CreateChallenge(&ChallengeConfig{ChallengeCount: 2, ChallengeSize: 8, ChallengeDifficulty: 1, Store: true})
			if err != nil {
				t.Fatal(err)
			}
			var answers []any
			for _, tuple := range challenge.Challenge {
				nonce := 0
				for {
					hash := sha256.Sum256([]byte(tuple[0] + strconv.Itoa(nonce)))
					if strings.HasPrefix(hex.EncodeToString(hash[:]), tuple[1]) {
						break
					}
					nonce++
				}
				if modern {
					answers = append(answers, nonce)
				} else {
					answers = append(answers, []any{tuple[0], tuple[1], nonce})
				}
			}
			wire, _ := json.Marshal(map[string]any{"token": challenge.Token, "solutions": answers})
			var solution Solution
			if err := json.Unmarshal(wire, &solution); err != nil {
				t.Fatal(err)
			}
			redeemed, err := server.RedeemChallenge(&solution)
			if err != nil || !redeemed.Success {
				t.Fatalf("redemption failed: %+v %v", redeemed, err)
			}
			if again, err := server.RedeemChallenge(&solution); err != nil || again.Success {
				t.Fatal("challenge replay was accepted")
			}
			validated, err := server.ValidateToken(redeemed.Token, &TokenConfig{})
			if err != nil || !validated.Success {
				t.Fatal("valid token rejected")
			}
			if again, err := server.ValidateToken(redeemed.Token, &TokenConfig{}); err != nil || again.Success {
				t.Fatal("token replay was accepted")
			}
		})
	}
}
