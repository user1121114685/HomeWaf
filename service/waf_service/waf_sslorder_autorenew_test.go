package waf_service

import (
	"errors"
	"testing"

	"HomeWaf/model"
	"HomeWaf/model/request"
)

func TestSSLOrderAutoRenewDefaultAndDuplicateSubmission(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.SslOrder{}, &model.SslRenewalPolicy{}); err != nil {
		t.Fatal(err)
	}
	enabled, disabled := true, false
	for _, tc := range []struct {
		name       string
		preference *bool
		want       bool
	}{
		{"default", nil, true}, {"enabled", &enabled, true}, {"disabled", &disabled, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			domain := tc.name + ".example.com"
			req := request.WafSslorderaddReq{RequestID: tc.name, ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", ApplyEmail: "owner@example.com", ApplyDomain: domain, AutoRenew: tc.preference}
			order, created, err := WafSSLOrderServiceApp.Submit(req, "")
			if err != nil || !created {
				t.Fatalf("submit: %v", err)
			}
			stored := WafSSLOrderServiceApp.GetDetailById(order.Id)
			if stored.AutoRenew == nil || stored.AutoRenewEnabled() != tc.want {
				t.Fatal("renewal preference did not persist")
			}
			req.AutoRenew = &tc.want
			if _, created, err := WafSSLOrderServiceApp.Submit(req, ""); err != nil || created {
				t.Fatalf("explicit default changed request identity: %v", err)
			}
			opposite := !tc.want
			req.AutoRenew = &opposite
			if _, _, err := WafSSLOrderServiceApp.Submit(req, ""); !errors.Is(err, ErrSSLOrderRequestConflict) {
				t.Fatalf("preference change reused submission key: %v", err)
			}
			if !tc.want {
				if _, created, err := WafSSLOrderServiceApp.AutomaticRenew(stored); err != nil || created {
					t.Fatalf("opted-out order renewed: %v", err)
				}
			}
		})
	}
	rows, count, err := WafSSLOrderServiceApp.GetListApi(request.WafSslordersearchReq{})
	if err != nil || count != 3 || len(rows) != 3 {
		t.Fatalf("status filter: rows=%d total=%d err=%v", len(rows), count, err)
	}
}
