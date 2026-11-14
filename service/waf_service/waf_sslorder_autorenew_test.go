package waf_service

import (
	"errors"
	"testing"
	"time"

	"HomeWaf/customtype"
	"HomeWaf/model"
	"HomeWaf/model/request"
)

func TestSSLOrderAutoRenewDefaultOptOutAndInheritance(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.SslOrder{}); err != nil {
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
			req := request.WafSslorderaddReq{RequestID: tc.name, HostCode: tc.name, ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", ApplyEmail: "owner@example.com", ApplyDomain: "example.com", AutoRenew: tc.preference}
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
			stored.ApplyStatus = "success"
			stored.ResultCertificate, stored.ResultPrivateKey = []byte("fixture-cert"), []byte("fixture-key")
			stored.ResultValidTo = customtype.JsonTime(time.Now().Add(7 * 24 * time.Hour))
			if err := WafSSLOrderServiceApp.ModifyById(stored); err != nil {
				t.Fatal(err)
			}
			if !tc.want {
				if _, created, err := WafSSLOrderServiceApp.AutomaticRenew(stored); err != nil || created {
					t.Fatalf("opted-out order renewed: %v", err)
				}
			}
			child, created, err := WafSSLOrderServiceApp.SubmitRenew(request.WafSslordereditReq{Id: stored.Id, RequestID: "renew-" + tc.name, HostCode: stored.HostCode, ApplyPlatform: stored.ApplyPlatform, ApplyMethod: stored.ApplyMethod, ApplyDns: stored.ApplyDns, ApplyEmail: stored.ApplyEmail, ApplyDomain: stored.ApplyDomain})
			if err != nil || !created || child.AutoRenewEnabled() != tc.want {
				t.Fatalf("renewal lost original preference: %v", err)
			}
		})
	}
	rows, count, err := WafSSLOrderServiceApp.GetListApi(request.WafSslordersearchReq{ApplyStatus: "success"})
	if err != nil || count != 3 || len(rows) != 3 {
		t.Fatalf("status filter: rows=%d total=%d err=%v", len(rows), count, err)
	}
}
