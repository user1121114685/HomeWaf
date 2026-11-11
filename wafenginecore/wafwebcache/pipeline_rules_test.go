package wafwebcache

import (
	"HomeWaf/model"
	"HomeWaf/model/wafenginmodel"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

func TestCacheRuleMatchingNeverMutatesPublishedRules(t *testing.T) {
	rules := []model.CacheRule{{RuleName: "fallback", RuleType: 2, RuleContent: "/", ParamType: 2, CacheTime: 60, Priority: 1}, {RuleName: "specific", RuleType: 3, RuleContent: "/public", ParamType: 2, CacheTime: 10, Priority: 10}}
	original := append([]model.CacheRule{}, rules...)
	host := &wafenginmodel.HostSafe{CacheRule: rules}
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for n := 0; n < 10; n++ {
				ttl, err := checkCacheRule(host, model.CacheConfig{}, httptest.NewRequest("GET", "http://family.test/public", nil))
				if err != nil || ttl != 10 {
					t.Error("cache priority changed", ttl, err)
				}
			}
		}()
	}
	workers.Wait()
	if !reflect.DeepEqual(original, host.CacheRule) {
		t.Fatal("request reordered immutable cache rules")
	}
}
