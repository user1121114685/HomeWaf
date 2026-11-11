package utils

import (
	"HomeWaf/innerbean"
	"sync"
	"testing"
)

func TestRuleMatchingIsolatesConcurrentExecutionState(t *testing.T) {
	helper := &RuleHelper{}
	helper.InitRuleEngine()
	if err := helper.LoadRuleString(`rule isolated "isolated" salience 10 { when MF.SRC_IP == "192.0.2.1" then MF.RISK_LEVEL = 2; }`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				ip := "198.51.100.1"
				want := 0
				if (worker+i)%2 == 0 {
					ip = "192.0.2.1"
					want = 1
				}
				matches, err := helper.Match("MF", &innerbean.WebLog{SRC_IP: ip})
				if err != nil || len(matches) != want {
					t.Errorf("cross-request result: want=%d matches=%d err=%v", want, len(matches), err)
					return
				}
			}
		}(worker)
	}
	wg.Wait()
}
