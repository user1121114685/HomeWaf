package waf_service

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/model/spec"
)

func TestSSLOrderConcurrentSubmissionRetryAndClaim(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.SslOrder{}); err != nil {
		t.Fatal(err)
	}
	req := request.WafSslorderaddReq{RequestID: "test-request", HostCode: "nas", ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", PrivateGroupName: "household", ApplyEmail: "owner@example.com", ApplyDomain: "example.com,*.example.com"}
	var created atomic.Int32
	var wg sync.WaitGroup
	ids := make(chan string, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			order, added, err := WafSSLOrderServiceApp.Submit(req, "")
			if err != nil {
				t.Error(err)
				return
			}
			if added {
				created.Add(1)
			}
			ids <- order.Id
		}()
	}
	wg.Wait()
	close(ids)
	if created.Load() != 1 {
		t.Fatalf("created %d orders", created.Load())
	}
	var id string
	for got := range ids {
		if id != "" && id != got {
			t.Fatal("retry returned another order")
		}
		id = got
	}
	var count int64
	if err := db.Model(&model.SslOrder{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("rows %d: %v", count, err)
	}
	req.ApplyDomain = "*.EXAMPLE.COM, example.com."
	if _, added, err := WafSSLOrderServiceApp.Submit(req, ""); err != nil || added {
		t.Fatalf("domain set normalization: %v, added %v", err, added)
	}
	req.ApplyEmail = "different@example.com"
	if _, _, err := WafSSLOrderServiceApp.Submit(req, ""); !errors.Is(err, ErrSSLOrderRequestConflict) {
		t.Fatalf("different payload reused key: %v", err)
	}
	req.RequestID = "second-tab"
	if _, _, err := WafSSLOrderServiceApp.Submit(req, ""); !errors.Is(err, ErrSSLOrderActive) {
		t.Fatalf("active service accepted duplicate: %v", err)
	}
	if err := WafSSLOrderServiceApp.DelApi(request.WafSslorderdeleteReq{Id: id}); err == nil {
		t.Fatal("pending work deleted")
	}
	var claims atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := WafSSLOrderServiceApp.Claim(id)
			if err != nil {
				t.Error(err)
			}
			if ok {
				claims.Add(1)
			}
		}()
	}
	wg.Wait()
	if claims.Load() != 1 {
		t.Fatalf("claimed %d times", claims.Load())
	}
	order := WafSSLOrderServiceApp.GetDetailById(id)
	if order.ApplyStatus != "applying" || order.Progress == "" {
		t.Fatalf("worker has no visible progress: %s", order.ApplyStatus)
	}
	order.ApplyStatus = "fail"
	if err := WafSSLOrderServiceApp.ModifyById(order); err != nil {
		t.Fatal(err)
	}
	req.ApplyEmail, req.RequestID = "owner@example.com", "test-request"
	if retry, added, err := WafSSLOrderServiceApp.Submit(req, ""); err != nil || added || retry.Id != id {
		t.Fatalf("lost-response retry created work: %v", err)
	}
	req.RequestID = "explicit-new-attempt"
	if _, added, err := WafSSLOrderServiceApp.Submit(req, ""); err != nil || !added {
		t.Fatalf("explicit retry after failure rejected: %v", err)
	}
	// The queue may be full; durable pending work remains available to the scanner.
	oldQueue := global.GWAF_CHAN_SSLOrder
	global.GWAF_CHAN_SSLOrder = make(chan spec.ChanSslOrder)
	t.Cleanup(func() { global.GWAF_CHAN_SSLOrder = oldQueue })
	done := make(chan struct{})
	go func() { WakeSSLOrderWorker(order); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("full queue blocked HTTP submission")
	}
	if pending, err := WafSSLOrderServiceApp.Pending(); err != nil || len(pending) != 1 {
		t.Fatalf("durable pending work missing: %d %v", len(pending), err)
	}
}

func TestSSLOrderRenewalIsAtomicAndDoesNotRewriteSource(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.SslOrder{}); err != nil {
		t.Fatal(err)
	}
	req := request.WafSslorderaddReq{RequestID: "source", HostCode: "nas", ApplyPlatform: "letsencrypt", ApplyMethod: "dns01", ApplyDns: "hurricane", ApplyEmail: "owner@example.com", ApplyDomain: "example.com"}
	order, _, err := WafSSLOrderServiceApp.Submit(req, "")
	if err != nil {
		t.Fatal(err)
	}
	order.ApplyStatus = "success"
	order.ResultCertificate, order.ResultPrivateKey = []byte("fixture-cert"), []byte("fixture-private-secret")
	order.ApplyKey = "fixture-account-secret"
	order.ResultValidTo = customtype.JsonTime(time.Now().Add(24 * time.Hour))
	if err := WafSSLOrderServiceApp.ModifyById(order); err != nil {
		t.Fatal(err)
	}
	req.RequestID, req.SkipDNSVerify = "renew", 1
	var wg sync.WaitGroup
	var created atomic.Int32
	var renewalID string
	var mu sync.Mutex
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			child, added, err := WafSSLOrderServiceApp.Submit(req, order.Id)
			if err != nil {
				t.Error(err)
				return
			}
			if added {
				created.Add(1)
			}
			mu.Lock()
			renewalID = child.Id
			mu.Unlock()
		}()
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("created %d renewals", created.Load())
	}
	source := WafSSLOrderServiceApp.GetDetailById(order.Id)
	if source.ApplyStatus != "success" || source.SkipDNSVerify != 0 {
		t.Fatal("pending renewal rewrote successful source")
	}
	child := WafSSLOrderServiceApp.GetDetailById(renewalID)
	encoded, _ := json.Marshal(child)
	if strings.Contains(string(encoded), "private_key") || strings.Contains(string(encoded), "apply_key") || strings.Contains(string(encoded), "fixture-account-secret") {
		t.Fatal("order API leaks private keys")
	}
	child.ApplyStatus = "success"
	if err := WafSSLOrderServiceApp.ModifyById(child); err != nil {
		t.Fatal(err)
	}
	if WafSSLOrderServiceApp.GetDetailById(order.Id).ApplyStatus != "renewed" {
		t.Fatal("successful renewal did not retire source")
	}
	if retry, added, err := WafSSLOrderServiceApp.Submit(req, order.Id); err != nil || added || retry.Id != renewalID {
		t.Fatalf("renewal response-loss retry: %v", err)
	}
}

func TestSSLOrderLegacyDuplicateAndInterruptedProgress(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.SslOrder{}); err != nil {
		t.Fatal(err)
	}
	first := model.SslOrder{HostCode: "nas", ApplyStatus: "submitted", ApplyDomain: "example.com", ApplyEmail: "owner@example.com"}
	first.Id, first.CREATE_TIME = "legacy-a", customtype.JsonTime(time.Now())
	second := first
	second.Id = "legacy-b"
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	if _, ok, err := WafSSLOrderServiceApp.Claim(second.Id); err != nil || ok {
		t.Fatalf("duplicate reached CA: %v", err)
	}
	if duplicate := WafSSLOrderServiceApp.GetDetailById(second.Id); duplicate.ApplyStatus != "fail" || !strings.Contains(duplicate.ResultError, first.Id) {
		t.Fatal("duplicate history not explained")
	}
	if _, ok, err := WafSSLOrderServiceApp.Claim(first.Id); err != nil || !ok {
		t.Fatalf("original not processed: %v", err)
	}
	if err := WafSSLOrderServiceApp.FailInterrupted(time.Now()); err != nil {
		t.Fatal(err)
	}
	if WafSSLOrderServiceApp.GetDetailById(first.Id).ApplyStatus != "applying" {
		t.Fatal("live heartbeat treated as interrupted")
	}
	if err := WafSSLOrderServiceApp.FailInterrupted(time.Now().Add(11 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if WafSSLOrderServiceApp.GetDetailById(first.Id).ApplyStatus != "fail" {
		t.Fatal("interrupted job stuck in progress")
	}
}
