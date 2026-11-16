package waf_service

import (
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/request"
	"HomeWaf/utils"
	"fmt"
	"github.com/go-acme/lego/v4/acme"
	"strings"
	"testing"
	"time"
)

func TestCertificateLifecyclePolicyHistoryAndRetry(t *testing.T) {
	db := setupSslExportTestDB(t)
	for _, m := range []any{&model.Hosts{}, &model.SslOrder{}, &model.SslOrderLog{}, &model.SslRenewalPolicy{}, &model.CertificateEvent{}, &model.CertificateDelivery{}} {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	base := func(id string) baseorm.BaseOrm {
		return baseorm.BaseOrm{Id: id, USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: customtype.JsonTime(now), UPDATE_TIME: customtype.JsonTime(now)}
	}
	cert, key := genExportTestCert(t, "nas.example.test")
	c := model.SslConfig{BaseOrm: base("cert"), AutoLoadPath: 0}
	if err := c.FillByCertAndKey(cert, key); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	h := model.Hosts{BaseOrm: base("host"), Code: "nas", Host: "nas.example.test", Port: 80, Ssl: 0, BindSslId: c.Id, Certfile: cert, Keyfile: key}
	if err := db.Create(&h).Error; err != nil {
		t.Fatal(err)
	}
	enabled := true
	source := model.SslOrder{BaseOrm: base("source"), HostCode: h.Code, ApplyDomain: h.Host, ApplyPlatform: "letsencrypt", ApplyMethod: "http01", ApplyEmail: "owner@example.test", AutoRenew: &enabled, ApplyStatus: "success", ApplyKey: "private-account-material", ResultCertificate: []byte(cert), ResultPrivateKey: []byte(key), ResultValidTo: c.ValidTo}
	if err := db.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateCertificatePolicies(); err != nil {
		t.Fatal(err)
	}
	var p model.SslRenewalPolicy
	if err := db.First(&p, "id = ?", c.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !p.Enabled || p.Version != 1 || p.HostCode != "nas" {
		t.Fatalf("bad migration: %+v", p)
	}
	var raw string
	db.Raw("SELECT issuance_context FROM ssl_renewal_policies WHERE id = ?", p.Id).Scan(&raw)
	if strings.Contains(raw, "private-account-material") {
		t.Fatal("ACME context stored in plaintext")
	}
	if err := WafSSLOrderServiceApp.DelApi(request.WafSslorderdeleteReq{Id: source.Id}); err != nil {
		t.Fatal(err)
	}
	job, created, err := RequestCertificateRenewal(p.Id, "manual-one", false, now)
	if err != nil || !created || job.ApplyKey != source.ApplyKey {
		t.Fatalf("history deletion lost context: %v", err)
	}
	again, created, err := RequestCertificateRenewal(p.Id, "manual-one", false, now)
	if err != nil || created || again.Id != job.Id {
		t.Fatalf("duplicate renewal: %v", err)
	}
	if _, claimed, err := WafSSLOrderServiceApp.Claim(job.Id); err != nil || !claimed {
		t.Fatalf("claim: %v", err)
	}
	job.Stage = "issue"
	if err := RecordCertificateFailure(job, testCertificateError("connection timeout"), now); err != nil {
		t.Fatal(err)
	}
	db.First(&p, "id = ?", p.Id)
	if p.Status != "retry_wait" || !time.Time(p.NextAttemptAt).After(now.Add(14*time.Minute)) {
		t.Fatalf("retry not persisted: %+v", p)
	}
	var eventCount int64
	db.Model(&model.CertificateEvent{}).Count(&eventCount)
	if eventCount != 1 {
		t.Fatal("failure notification not durable")
	}
	before := p.Version
	p, err = SaveCertificatePolicy(CertificatePolicyEdit{ID: p.Id, Version: p.Version, Enabled: false, Method: "http01", Email: p.ApplyEmail})
	if err != nil || p.Enabled || p.Version != before+1 {
		t.Fatalf("disable: %v", err)
	}
	var count int64
	db.Model(&model.SslOrder{}).Count(&count)
	if count != 1 {
		t.Fatal("saving strategy created a job")
	}
	if _, err := SaveCertificatePolicy(CertificatePolicyEdit{ID: p.Id, Version: before, Enabled: true, Method: "http01", Email: p.ApplyEmail}); err != ErrCertificateConflict {
		t.Fatal("stale update accepted", err)
	}
	if err := RunCertificateRenewals(now.Add(time.Hour), true); err != nil {
		t.Fatal(err)
	}
	db.Model(&model.SslOrder{}).Count(&count)
	if count != 1 {
		t.Fatal("disabled policy renewed")
	}
	view, err := CertificateDetail(c.Id)
	if err != nil || view.KeyContent != "" || view.Policy == nil {
		t.Fatal("detail leaked key or lost policy", err)
	}
}

type testCertificateError string

func (e testCertificateError) Error() string { return string(e) }

func TestCertificateLifecycleRateLimitsAndRetryBudget(t *testing.T) {
	db := setupSslExportTestDB(t)
	if err := db.AutoMigrate(&model.Hosts{}, &model.SslOrder{}, &model.SslOrderLog{}, &model.SslRenewalPolicy{}, &model.CertificateEvent{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Second)
	base := func(id string) baseorm.BaseOrm {
		return baseorm.BaseOrm{Id: id, USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: customtype.JsonTime(now), UPDATE_TIME: customtype.JsonTime(now)}
	}
	cert, key := genExportTestCert(t, "retry.example.test")
	c := model.SslConfig{BaseOrm: base("retry-cert")}
	if err := c.FillByCertAndKey(cert, key); err != nil {
		t.Fatal(err)
	}
	h := model.Hosts{BaseOrm: base("retry-host"), Code: "retry", Host: "retry.example.test", Port: 80, BindSslId: c.Id, Certfile: cert, Keyfile: key}
	source := model.SslOrder{HostCode: h.Code, ApplyDomain: h.Host, ApplyPlatform: "letsencrypt", ApplyMethod: "http01", ApplyEmail: "owner@example.test"}
	p := newCertificatePolicy(c, source, h.Id)
	for _, row := range []any{&c, &h, &p} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	at := now
	for attempt, delay := range []time.Duration{15 * time.Minute, time.Hour, 4 * time.Hour, 6 * time.Hour, 6 * time.Hour, 6 * time.Hour} {
		job, created, err := RequestCertificateRenewal(c.Id, fmt.Sprintf("attempt-%d", attempt), true, at)
		if err != nil || !created {
			t.Fatalf("automatic attempt %d not created: %v", attempt+1, err)
		}
		if err := RecordCertificateFailure(job, testCertificateError("connection timeout"), at); err != nil {
			t.Fatal(err)
		}
		if err := db.First(&p, "id = ?", c.Id).Error; err != nil {
			t.Fatal(err)
		}
		wait := time.Time(p.NextAttemptAt).Sub(at)
		if p.Status != "retry_wait" || wait < delay || wait > delay+time.Minute {
			t.Fatalf("attempt %d lost persisted backoff: %s / %v", attempt+1, p.Status, wait)
		}
		at = time.Time(p.NextAttemptAt)
	}
	if _, created, err := RequestCertificateRenewal(c.Id, "over-budget", true, at); err != nil || created {
		t.Fatalf("seventh attempt within 24 hours was accepted: %v", err)
	}
	if err := db.First(&p, "id = ?", c.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !p.Enabled || p.Status != "retry_wait" || !time.Time(p.NextAttemptAt).After(now.Add(24*time.Hour)) {
		t.Fatal("budget must defer, not disable, automatic renewal")
	}
	at = time.Time(p.NextAttemptAt)
	job, created, err := RequestCertificateRenewal(c.Id, "new-window", true, at)
	if err != nil || !created {
		t.Fatalf("automatic attempts did not resume in the new window: %v", err)
	}
	limited := &acme.RateLimitedError{ProblemDetails: &acme.ProblemDetails{HTTPStatus: 429, Type: "urn:ietf:params:acme:error:rateLimited"}, RetryAfter: "28800"}
	if err := RecordCertificateFailure(job, limited, at); err != nil {
		t.Fatal(err)
	}
	if _, created, err := RequestCertificateRenewal(c.Id, "manual-during-limit", false, at.Add(time.Hour)); err == nil || created {
		t.Fatal("manual renewal bypassed the CA rate limit")
	}
	if err := db.First(&p, "id = ?", c.Id).Error; err != nil {
		t.Fatal(err)
	}
	if p.FailureKind != "rate_limit" || time.Time(p.NextAttemptAt).Before(at.Add(8*time.Hour)) {
		t.Fatal("CA Retry-After was not preserved")
	}
}

func TestCertificateLifecycleInitialSubmissionWithSingleConnection(t *testing.T) {
	db := setupSslExportTestDB(t)
	for _, m := range []any{&model.Hosts{}, &model.SslOrder{}, &model.SslOrderLog{}, &model.SslRenewalPolicy{}} {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatal(err)
		}
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	host := model.Hosts{BaseOrm: baseorm.BaseOrm{Id: "single-host", USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID}, Code: "single", Host: "single.example.test", Port: 80}
	if err := db.Create(&host).Error; err != nil {
		t.Fatal(err)
	}
	order, created, err := WafSSLOrderServiceApp.Submit(request.WafSslorderaddReq{RequestID: "initial-single", HostCode: host.Code, ApplyDomain: host.Host, ApplyMethod: "http01", ApplyPlatform: "letsencrypt", ApplyEmail: "owner@example.test"}, "")
	if err != nil || !created || order.Operation != "issuance" || order.BindingSnapshot == "" {
		t.Fatalf("initial submission not persisted: %v", err)
	}
	if _, claimed, err := WafSSLOrderServiceApp.Claim(order.Id); err != nil || !claimed {
		t.Fatalf("new order not claimable: %v", err)
	}
}

func TestCertificateLifecycleDeploymentAndEncryptedMaterial(t *testing.T) {
	db := setupSslExportTestDB(t)
	for _, m := range []any{&model.Hosts{}, &model.SslOrder{}, &model.SslOrderLog{}, &model.SslRenewalPolicy{}, &model.CertificateEvent{}, &model.CertificateDelivery{}} {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	base := func(id string) baseorm.BaseOrm {
		return baseorm.BaseOrm{Id: id, USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: customtype.JsonTime(now), UPDATE_TIME: customtype.JsonTime(now)}
	}
	original, key := genExportTestCert(t, "nas.example.test")
	h := model.Hosts{BaseOrm: base("host"), Code: "nas", Host: "nas.example.test", Port: 80, Certfile: original, Keyfile: key}
	if err := db.Create(&h).Error; err != nil {
		t.Fatal(err)
	}
	enabled := true
	job := model.SslOrder{BaseOrm: base("initial"), HostCode: h.Code, ApplyDomain: h.Host, ApplyPlatform: "letsencrypt", ApplyMethod: "http01", ApplyEmail: "owner@example.test", AutoRenew: &enabled, ApplyStatus: "applying", ApplyKey: "account-key-private"}
	if err := PrepareInitialCertificateOrder(&job); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	certificate, newKey := genExportTestCert(t, "nas.example.test")
	var parsed model.SslConfig
	if err := parsed.FillByCertAndKey(certificate, newKey); err != nil {
		t.Fatal(err)
	}
	job.ResultCertificate, job.ResultPrivateKey, job.ResultValidTo = []byte(certificate), []byte(newKey), parsed.ValidTo
	if err := RecordIssuedCertificate(&job); err != nil {
		t.Fatal(err)
	}
	var ciphertext string
	db.Raw("SELECT result_private_key FROM ssl_orders WHERE id = ?", job.Id).Scan(&ciphertext)
	if strings.Contains(ciphertext, "PRIVATE KEY") {
		t.Fatal("issued key stored in plaintext")
	}
	oldQueue := global.GWAF_CHAN_MSG
	global.GWAF_CHAN_MSG = nil
	t.Cleanup(func() { global.GWAF_CHAN_MSG = oldQueue })
	if err := DeployCertificateOrder(&job); err != nil {
		t.Fatal(err)
	}
	if err := CompleteCertificateOrder(job, now); err != nil {
		t.Fatal(err)
	}
	var stored model.Hosts
	db.First(&stored, "id = ?", h.Id)
	if stored.BindSslId != job.CertificateID || stored.Certfile != certificate || stored.Keyfile != newKey {
		t.Fatal("certificate not bound to intended host")
	}
	if stored.Ssl != h.Ssl || stored.Port != h.Port {
		t.Fatal("issuance changed listener protocol")
	}
	var policy model.SslRenewalPolicy
	db.First(&policy, "id = ?", job.CertificateID)
	db.Raw("SELECT issuance_context FROM ssl_renewal_policies WHERE id = ?", policy.Id).Scan(&ciphertext)
	if strings.Contains(ciphertext, "account-key-private") {
		t.Fatal("policy context update stored plaintext")
	}
	if err := WafSSLOrderServiceApp.DelApi(request.WafSslorderdeleteReq{Id: job.Id}); err != nil {
		t.Fatal(err)
	}
	renewed, created, err := RequestCertificateRenewal(policy.Id, "after-history-delete", false, now)
	if err != nil || !created || renewed.ApplyKey != "account-key-private" {
		t.Fatalf("renew after history removal: %v", err)
	}
	// A later manual certificate must not be overwritten by an in-flight job.
	different, otherKey := genExportTestCert(t, "nas.example.test")
	db.Model(&model.SslConfig{}).Where("id = ?", policy.Id).Update("cert_content", different)
	renewed.ResultCertificate, renewed.ResultPrivateKey = []byte(certificate), []byte(newKey)
	if err := DeployCertificateOrder(&renewed); err == nil {
		t.Fatal("in-flight job overwrote a manual replacement")
	}
	_ = otherKey
}
func TestCertificateLifecycleNotificationRetryAndMonitoringState(t *testing.T) {
	db := setupSslExportTestDB(t)
	for _, m := range []any{&model.SslRenewalPolicy{}, &model.CertificateEvent{}, &model.CertificateDelivery{}, &model.NotifyChannel{}, &model.NotifySubscription{}, &model.NotifyLog{}, &model.SystemConfig{}} {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatal(err)
		}
	}
	oldLog, oldDeliver := global.GWAF_LOCAL_LOG_DB, certificateDeliver
	global.GWAF_LOCAL_LOG_DB = db
	t.Cleanup(func() { global.GWAF_LOCAL_LOG_DB = oldLog; certificateDeliver = oldDeliver })
	now := time.Now()
	base := func(id string) baseorm.BaseOrm {
		return baseorm.BaseOrm{Id: id, USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: customtype.JsonTime(now), UPDATE_TIME: customtype.JsonTime(now)}
	}
	channel := model.NotifyChannel{BaseOrm: base("notify-channel"), Name: "Test", Type: "webhook", Status: 1}
	sub := model.NotifySubscription{BaseOrm: base("notify-sub"), ChannelId: channel.Id, MessageType: model.MSG_TYPE_OPERATION_NOTICE, Status: 1, ThrottleMode: "realtime"}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}
	event := model.CertificateEvent{BaseOrm: base("event"), CertificateID: "certificate", OrderID: "order", Kind: "failure", Severity: "warn", Domain: "nas.example.test", Message: "Temporary failure"}
	if err := db.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	sends := 0
	certificateDeliver = func(model.NotifyChannel, model.NotifySubscription, string, string, string) (string, error) {
		sends++
		if sends == 1 {
			return "", testCertificateError("provider temporarily unavailable")
		}
		return "test-recipient", nil
	}
	if err := DispatchCertificateNotifications(now); err != nil {
		t.Fatal(err)
	}
	var delivery model.CertificateDelivery
	if err := db.First(&delivery, "event_id = ?", event.Id).Error; err != nil {
		t.Fatal(err)
	}
	if delivery.Status != "retry_wait" || delivery.Attempts != 1 {
		t.Fatalf("failed notification not queued: %+v", delivery)
	}
	if err := DispatchCertificateNotifications(now.Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if sends != 1 {
		t.Fatal("notification retried before backoff")
	}
	if err := DispatchCertificateNotifications(now.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	db.First(&delivery, "id = ?", delivery.Id)
	if delivery.Status != "sent" || sends != 2 {
		t.Fatal("notification did not recover")
	}
	if err := DispatchCertificateNotifications(now.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if sends != 2 {
		t.Fatal("delivered event sent twice")
	}
	sub.ThrottleMode = "aggregate"
	if err := db.Model(&sub).Update("throttle_mode", "aggregate").Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"aggregate-two", "aggregate-three"} {
		e := event
		e.Id = id
		e.Domain = id + ".example.test"
		e.Message = id
		e.Expanded = false
		e.CREATE_TIME = customtype.JsonTime(now.Add(10 * time.Minute))
		if err := db.Create(&e).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DispatchCertificateNotifications(now.Add(10 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if sends != 2 {
		t.Fatal("aggregate window was ignored")
	}
	if err := DispatchCertificateNotifications(now.Add(11 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if sends != 3 {
		t.Fatalf("notifications were not coalesced into one delivery: %d", sends)
	}
	if err := db.Model(&sub).Update("throttle_mode", "realtime").Error; err != nil {
		t.Fatal(err)
	}
	limitedAt := now.Add(12 * time.Minute)
	limitedEvent := event
	limitedEvent.Id, limitedEvent.Expanded = "limited-event", false
	limitedEvent.CREATE_TIME = customtype.JsonTime(limitedAt)
	if err := db.Create(&limitedEvent).Error; err != nil {
		t.Fatal(err)
	}
	certificateDeliver = func(model.NotifyChannel, model.NotifySubscription, string, string, string) (string, error) {
		sends++
		return "", utils.NewHTTPRetryError(429, "3600", "test-channel", limitedAt)
	}
	if err := DispatchCertificateNotifications(limitedAt); err != nil {
		t.Fatal(err)
	}
	var limitedDelivery model.CertificateDelivery
	if err := db.First(&limitedDelivery, "event_id = ?", limitedEvent.Id).Error; err != nil {
		t.Fatal(err)
	}
	if time.Time(limitedDelivery.NextAttemptAt).Before(limitedAt.Add(time.Hour)) {
		t.Fatal("notification retry ignored the channel Retry-After")
	}
	if err := RetryCertificateNotification(limitedDelivery.Id); err == nil {
		t.Fatal("manual resend bypassed the channel rate limit")
	}
	if err := DispatchCertificateNotifications(limitedAt.Add(25 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&limitedDelivery, "id = ?", limitedDelivery.Id).Error; err != nil {
		t.Fatal(err)
	}
	if limitedDelivery.Status != "failed" || sends != 4 {
		t.Fatal("notification retry must stop after 24 hours and expose failure")
	}
	expired := model.SslExpire{ValidTo: customtype.JsonTime(now.Add(-48 * time.Hour)), LastDetect: customtype.JsonTime(now)}
	if expired.ExpirationDay() >= 0 || certificateMonitorState(expired, now) != "expired" {
		t.Fatal("expired certificate interpreted as future validity")
	}
	expired.VisitLog = "connection failed"
	if certificateMonitorState(expired, now) != "failed" {
		t.Fatal("failed probe hidden by old success")
	}
}
