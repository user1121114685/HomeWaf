package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCertificateLifecycleStandaloneIssueRenewAndRetry(t *testing.T) {
	db := setupSslExportTestDB(t)
	for _, entity := range []any{&model.PrivateInfo{}, &model.SslOrder{}, &model.SslOrderLog{}, &model.SslRenewalPolicy{}, &model.CertificateEvent{}, &model.CertificateDelivery{}} {
		if err := db.AutoMigrate(entity); err != nil {
			t.Fatal(err)
		}
	}
	oldQueue, oldMessages := global.GWAF_CHAN_SSLOrder, global.GWAF_CHAN_MSG
	global.GWAF_CHAN_SSLOrder, global.GWAF_CHAN_MSG = nil, nil
	t.Cleanup(func() { global.GWAF_CHAN_SSLOrder, global.GWAF_CHAN_MSG = oldQueue, oldMessages })
	if err := ValidateSSLCertificateTarget("", "dns01", "one.example.test"); err != nil {
		t.Fatal(err)
	}
	if ValidateSSLCertificateTarget("", "http01", "one.example.test") == nil {
		t.Fatal("HTTP challenge accepted without a serving endpoint")
	}
	if ValidateSSLCertificateTarget("deleted-service", "dns01", "one.example.test") == nil {
		t.Fatal("invalid selected service silently became independent")
	}
	create := func(domain, id string) model.SslOrder {
		t.Helper()
		if err := WafPrivateInfoServiceApp.AddApi(request.WafPrivateInfoAddReq{PrivateGroupBelongCloud: "hurricane", PrivateGroupName: id, PrivateKey: "HURRICANE_TOKENS", PrivateValue: domain + ":isolated-token"}); err != nil {
			t.Fatal(err)
		}
		input := request.WafSslorderaddReq{RequestID: id, ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", ApplyDomain: domain, ApplyEmail: "owner@example.test", PrivateGroupName: id}
		order, created, err := WafSSLOrderServiceApp.Submit(input, "")
		if err != nil || !created || order.HostCode != "" || order.BindingSnapshot != "[]" {
			t.Fatalf("independent submission: %+v %v", order, err)
		}
		again, created, err := WafSSLOrderServiceApp.Submit(input, "")
		if err != nil || created || again.Id != order.Id {
			t.Fatalf("duplicate independent request: %v", err)
		}
		input.RequestID += "-another-tab"
		if _, _, err := WafSSLOrderServiceApp.Submit(input, ""); !errors.Is(err, ErrSSLOrderActive) {
			t.Fatalf("same domain conflict not retained: %v", err)
		}
		return order
	}
	one, two := create("one.example.test", "one"), create("two.example.test", "two")
	if *one.ActiveKey == *two.ActiveKey {
		t.Fatal("unrelated domains share an active key")
	}
	issue := func(order model.SslOrder) model.SslOrder {
		t.Helper()
		claimed, ok, err := WafSSLOrderServiceApp.Claim(order.Id)
		if err != nil || !ok {
			t.Fatalf("claim: %v", err)
		}
		cert, key := genExportTestCert(t, order.ApplyDomain)
		var material model.SslConfig
		if err := material.FillByCertAndKey(cert, key); err != nil {
			t.Fatal(err)
		}
		claimed.ApplyKey = "isolated-acme-account"
		claimed.ResultCertificate, claimed.ResultPrivateKey, claimed.ResultValidTo = []byte(cert), []byte(key), material.ValidTo
		if err := RecordIssuedCertificate(&claimed); err != nil {
			t.Fatal(err)
		}
		return claimed
	}
	finish := func(order model.SslOrder) model.SslOrder {
		t.Helper()
		if err := DeployCertificateOrder(&order); err != nil {
			t.Fatal(err)
		}
		if err := CompleteCertificateOrder(order, time.Now()); err != nil {
			t.Fatal(err)
		}
		var policy model.SslRenewalPolicy
		if err := db.First(&policy, "id = ?", order.CertificateID).Error; err != nil {
			t.Fatal(err)
		}
		if policy.HostCode != "" || policy.TargetID != "" || policy.Status != "idle" {
			t.Fatalf("independent policy depends on service: %+v", policy)
		}
		bindings, err := CertificateBindings(db, order.CertificateID)
		if err != nil || len(bindings) != 0 {
			t.Fatalf("created an unselected service binding: %+v %v", bindings, err)
		}
		return order
	}
	issuedOne := issue(one)
	bad := issuedOne
	bad.HostCode = "not-selected"
	if DeployCertificateOrder(&bad) == nil {
		t.Fatal("selected service accepted an empty binding snapshot")
	}
	bad = issuedOne
	bad.BindingSnapshot = certificateSnapshot([]CertificateBinding{{Code: "not-selected", ID: "unrelated"}})
	if DeployCertificateOrder(&bad) == nil {
		t.Fatal("independent issuance accepted an unselected binding")
	}
	one, two = finish(issuedOne), finish(issue(two))
	var encrypted string
	db.Raw("SELECT key_content FROM ssl_configs WHERE id = ?", one.CertificateID).Scan(&encrypted)
	if strings.Contains(encrypted, "PRIVATE KEY") {
		t.Fatal("certificate key stored in plaintext")
	}
	renewOne, created, err := RequestCertificateRenewal(one.CertificateID, "manual-independent", false, time.Now())
	if err != nil || !created {
		t.Fatalf("independent manual renewal: %v", err)
	}
	renewTwo, created, err := RequestCertificateRenewal(two.CertificateID, "automatic-independent", true, time.Now())
	if err != nil || !created {
		t.Fatalf("unrelated independent renewal blocked: %v", err)
	}
	if *renewOne.ActiveKey == *renewTwo.ActiveKey {
		t.Fatal("independent renewals share an active key")
	}
	third := issue(create("three.example.test", "three"))
	if err := RecordCertificateFailure(third, errors.New("isolated persistence failure"), time.Now()); err != nil {
		t.Fatal(err)
	}
	var failed model.SslOrder
	if err := db.First(&failed, "id = ?", third.Id).Error; err != nil {
		t.Fatal(err)
	}
	retry, created, err := RetryIssuedCertificate(failed, "retry-issued-independent")
	if err != nil || !created || string(retry.ResultCertificate) != string(third.ResultCertificate) {
		t.Fatalf("issued retry lost material: %v", err)
	}
	retry, claimed, err := WafSSLOrderServiceApp.Claim(retry.Id)
	if err != nil || !claimed {
		t.Fatalf("issued retry claim: %v", err)
	}
	finish(retry)
	renewed := finish(issue(renewOne))
	if renewed.CertificateID != one.CertificateID || string(renewed.ResultCertificate) == string(one.ResultCertificate) {
		t.Fatal("renewal did not update the same resource")
	}
	var count int64
	db.Model(&model.SslConfig{}).Count(&count)
	if count != 3 {
		t.Fatalf("renewal duplicated certificate resources: %d", count)
	}
}
