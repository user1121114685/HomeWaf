// HomeWaf modification (2026-11-17): durable certificate notifications.
package waf_service

import (
	"HomeWaf/internal/acmelog"
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/platform/common/uuid"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

var certificateDeliver = func(ch model.NotifyChannel, sub model.NotifySubscription, typ, title, body string) (string, error) {
	return WafNotifySenderServiceApp.deliverToChannel(ch, sub, typ, title, body)
}

func certificateNotifyEvent(e model.CertificateEvent) NotifyEvent {
	typ := model.MSG_TYPE_OPERATION_NOTICE
	if e.Kind == "expiry" {
		typ = model.MSG_TYPE_SSL_EXPIRE
	}
	title := "证书续期与故障"
	if e.Kind == "success" {
		title = "证书更新完成"
	}
	if e.Kind == "recovered" {
		title = "证书已恢复"
	}
	if e.Kind == "expiry" {
		title = "证书到期风险"
	}
	body := e.Message
	if e.CertificateID != "" {
		body += "\n控制台路径：/#/waf-host/wafsslconfiglist/" + e.CertificateID
	}
	if e.CertificateID == "" && e.OrderID != "" {
		body += "\n控制台路径：/#/waf-host/wafsslOrderlist?order=" + e.OrderID
	}
	ev := newNotifyEvent(typ, title, body)
	ev.Severity, ev.Time = e.Severity, time.Time(e.CREATE_TIME)
	ev.Vars["Severity"], ev.Vars["Domain"], ev.Vars["OperaType"], ev.Vars["OperaCnt"] = e.Severity, e.Domain, title, body
	ev.Vars["CertificateID"], ev.Vars["OrderID"], ev.Vars["CertificateEvent"] = e.CertificateID, e.OrderID, e.Kind
	ev.DedupParts[model.DedupKeyDomain] = e.Domain
	ev.DedupParts[model.DedupKeyAttackType] = "certificate:" + e.Kind
	return ev
}
func expandCertificateEvent(e model.CertificateEvent, now time.Time) error {
	db := global.GWAF_LOCAL_DB
	ev := certificateNotifyEvent(e)
	var subs []model.NotifySubscription
	if err := db.Where("message_type = ?", ev.MessageType).Find(&subs).Error; err != nil {
		return err
	}
	decisions := map[string]ThrottleDecision{}
	for _, sub := range subs {
		if sub.Status == 1 {
			decisions[sub.Id] = WafNotifyThrottleServiceApp.Decide(sub, ev)
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var current model.CertificateEvent
		if err := tx.First(&current, "id = ?", e.Id).Error; err != nil {
			return err
		}
		if current.Expanded {
			return nil
		}
		coverage := "no_subscription"
		if len(subs) > 0 {
			coverage = "configured"
		}
		for _, sub := range subs {
			id := sslSubmissionHash([]string{e.Id, sub.Id})
			var existing model.CertificateDelivery
			err := tx.First(&existing, "id = ?", id).Error
			if err == nil {
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			d := model.CertificateDelivery{BaseOrm: baseorm.BaseOrm{Id: id, USER_CODE: e.USER_CODE, Tenant_ID: e.Tenant_ID, CREATE_TIME: customtype.JsonTime(now), UPDATE_TIME: customtype.JsonTime(now)}, EventID: e.Id, SubscriptionID: sub.Id, ChannelID: sub.ChannelId, Status: "pending", NextAttemptAt: customtype.JsonTime(now)}
			var channel model.NotifyChannel
			channelErr := tx.First(&channel, "id = ?", sub.ChannelId).Error
			if channelErr != nil && !errors.Is(channelErr, gorm.ErrRecordNotFound) {
				return channelErr
			}
			d.ChannelName = channel.Name
			if sub.Status != 1 || channelErr != nil || channel.Status != 1 {
				d.Status, d.Error = "disabled", "订阅或渠道未启用"
			}
			if d.Status == "pending" {
				decision := decisions[sub.Id]
				if decision.Action == ThrottleActionSuppress {
					d.Status, d.Error = "filtered", decision.Reason
				} else if decision.Action == ThrottleActionAggregate {
					d.NextAttemptAt = customtype.JsonTime(now.Add(time.Duration(decision.Effective.AggregateWindowSec) * time.Second))
				}
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&d).Error; err != nil {
				return err
			}
		}
		return tx.Model(&e).Updates(map[string]any{"expanded": true, "coverage": coverage, "update_time": customtype.JsonTime(now)}).Error
	})
}
func notificationBackoff(attempt int) time.Duration {
	delays := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour}
	if attempt <= 0 {
		return delays[0]
	}
	if attempt > len(delays) {
		return 6 * time.Hour
	}
	return delays[attempt-1]
}
func DispatchCertificateNotifications(now time.Time) error {
	if !CertificateLifecycleAvailable() {
		return nil
	}
	db := global.GWAF_LOCAL_DB
	var events []model.CertificateEvent
	if err := db.Where("expanded = ?", false).Order("create_time asc").Limit(100).Find(&events).Error; err != nil {
		return err
	}
	for _, event := range events {
		if err := expandCertificateEvent(event, now); err != nil {
			return err
		}
	}
	var rows []model.CertificateDelivery
	if err := db.Where("(status IN ? AND next_attempt_at <= ?) OR (status = ? AND lease_until <= ?)", []string{"pending", "retry_wait"}, customtype.JsonTime(now), "sending", customtype.JsonTime(now)).Order("create_time asc").Limit(100).Find(&rows).Error; err != nil {
		return err
	}
	groups := map[string][]model.CertificateDelivery{}
	keys := []string{}
	for _, row := range rows {
		key := row.Id
		var sub model.NotifySubscription
		if err := db.First(&sub, "id = ?", row.SubscriptionID).Error; err == nil && WafNotifyThrottleServiceApp.Resolve(sub).Mode == model.ThrottleModeAggregate {
			key = row.SubscriptionID + ":" + row.ChannelID
		}
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], row)
	}
	started := time.Now()
	for _, key := range keys {
		if err := dispatchCertificateGroup(groups[key], now.Add(time.Since(started))); err != nil {
			return err
		}
	}
	return nil
}
func dispatchCertificateGroup(rows []model.CertificateDelivery, now time.Time) error {
	if len(rows) == 0 {
		return nil
	}
	db := global.GWAF_LOCAL_DB
	var sub model.NotifySubscription
	var channel model.NotifyChannel
	subErr := db.First(&sub, "id = ?", rows[0].SubscriptionID).Error
	channelErr := db.First(&channel, "id = ?", rows[0].ChannelID).Error
	if subErr != nil && !errors.Is(subErr, gorm.ErrRecordNotFound) {
		return subErr
	}
	if channelErr != nil && !errors.Is(channelErr, gorm.ErrRecordNotFound) {
		return channelErr
	}
	if subErr != nil || channelErr != nil || sub.Status != 1 || channel.Status != 1 {
		for _, d := range rows {
			if err := db.Model(&d).Updates(map[string]any{"status": "disabled", "error": "订阅或渠道未启用/已删除"}).Error; err != nil {
				return err
			}
		}
		return nil
	}
	effective := WafNotifyThrottleServiceApp.Resolve(sub)
	claimed := []model.CertificateDelivery{}
	events := []NotifyEvent{}
	lease := customtype.JsonTime(now.Add(2 * time.Minute))
	for _, d := range rows {
		if now.Sub(time.Time(d.CREATE_TIME)) >= 24*time.Hour {
			if err := db.Model(&d).Updates(map[string]any{"status": "failed", "error": "投递重试已超过24小时，请检查渠道后重新投递", "next_attempt_at": nil, "lease_until": nil}).Error; err != nil {
				return err
			}
			continue
		}
		var event model.CertificateEvent
		if err := db.First(&event, "id = ?", d.EventID).Error; err != nil {
			return err
		}
		ev := certificateNotifyEvent(event)
		if !MatchNotifyFilter(sub.GetFilterConfig(), ev) || (isInQuietHours(effective.QuietHours, now) && !severityBypass(effective.QuietBypass, ev.Severity)) {
			if err := db.Model(&d).Updates(map[string]any{"status": "filtered", "error": "当前过滤或免打扰规则不允许发送"}).Error; err != nil {
				return err
			}
			continue
		}
		claim := db.Model(&model.CertificateDelivery{}).Where("id = ? AND ((status IN ? AND next_attempt_at <= ?) OR (status = ? AND lease_until <= ?))", d.Id, []string{"pending", "retry_wait"}, customtype.JsonTime(now), "sending", customtype.JsonTime(now)).
			Updates(map[string]any{"status": "sending", "lease_until": lease, "attempts": d.Attempts + 1})
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected != 1 {
			continue
		}
		claimed = append(claimed, d)
		events = append(events, ev)
	}
	if len(claimed) == 0 {
		return nil
	}
	title, body, templateUsed := BuildMergedMessage(sub, channel.Type, events, effective.AggregateMaxDetail)
	if global.GWAF_NOTICE_TITLE != "" {
		title = "[" + global.GWAF_NOTICE_TITLE + "] " + title
	}
	recipients, sendErr := certificateDeliver(channel, sub, events[0].MessageType, title, body)
	logStatus, reason := 1, ""
	if sendErr != nil {
		logStatus = 0
		reason = acmelog.Sanitize(sendErr.Error())
	}
	for _, d := range claimed {
		status, next, notBefore := "sent", time.Time{}, time.Time{}
		if sendErr != nil {
			status = "retry_wait"
			next = now.Add(notificationBackoff(d.Attempts + 1))
			var limited interface{ RetryAt() time.Time }
			if errors.As(sendErr, &limited) {
				notBefore = limited.RetryAt()
				if notBefore.After(next) {
					next = notBefore
				}
			}
		}
		if err := db.Model(&model.CertificateDelivery{}).Where("id = ? AND status = ? AND lease_until = ?", d.Id, "sending", lease).
			Updates(map[string]any{"status": status, "error": reason, "retry_not_before": customtype.JsonTime(notBefore), "next_attempt_at": customtype.JsonTime(next), "lease_until": nil, "update_time": customtype.JsonTime(now)}).Error; err != nil {
			return err
		}
	}
	if global.GWAF_LOCAL_LOG_DB != nil {
		_, _ = WafNotifyLogServiceApp.AddLogDetail(model.NotifyLog{ChannelId: channel.Id, ChannelName: channel.Name, ChannelType: channel.Type, MessageType: events[0].MessageType, MessageTitle: title, MessageContent: body, Recipients: recipients, Status: logStatus, ErrorMsg: reason, SubscriptionId: sub.Id, TemplateUsed: templateUsed})
	}
	return nil
}

type CertificateNotificationView struct {
	Events                 []model.CertificateEvent    `json:"events"`
	Deliveries             []model.CertificateDelivery `json:"deliveries"`
	OperationSubscriptions int64                       `json:"operation_subscriptions"`
	ExpirySubscriptions    int64                       `json:"expiry_subscriptions"`
}

func CertificateNotifications(id string) (CertificateNotificationView, error) {
	return certificateNotificationState(id, "")
}
func CertificateOrderNotifications(id string) (CertificateNotificationView, error) {
	return certificateNotificationState("", id)
}
func certificateNotificationState(id, orderID string) (CertificateNotificationView, error) {
	db := global.GWAF_LOCAL_DB
	view := CertificateNotificationView{Events: []model.CertificateEvent{}, Deliveries: []model.CertificateDelivery{}}
	q := db.Order("create_time desc").Limit(30)
	if id != "" {
		q = q.Where("certificate_id = ?", id)
	} else if orderID != "" {
		q = q.Where("order_id = ?", orderID)
	}
	if err := q.Find(&view.Events).Error; err != nil {
		return view, err
	}
	ids := []string{}
	for _, e := range view.Events {
		ids = append(ids, e.Id)
	}
	if len(ids) > 0 {
		if err := db.Where("event_id IN ?", ids).Find(&view.Deliveries).Error; err != nil {
			return view, err
		}
	}
	for typ, target := range map[string]*int64{model.MSG_TYPE_OPERATION_NOTICE: &view.OperationSubscriptions, model.MSG_TYPE_SSL_EXPIRE: &view.ExpirySubscriptions} {
		var subs []model.NotifySubscription
		if err := db.Where("message_type = ? AND status = ?", typ, 1).Find(&subs).Error; err != nil {
			return view, err
		}
		for _, sub := range subs {
			var n int64
			if err := db.Model(&model.NotifyChannel{}).Where("id = ? AND status = ?", sub.ChannelId, 1).Count(&n).Error; err != nil {
				return view, err
			}
			*target += n
		}
	}
	return view, nil
}
func RetryCertificateNotification(id string) error {
	db := global.GWAF_LOCAL_DB
	var d model.CertificateDelivery
	if err := db.First(&d, "id = ?", id).Error; err != nil {
		return err
	}
	if time.Time(d.RetryNotBefore).After(time.Now()) {
		return fmt.Errorf("渠道限流，最早重试时间：%s", time.Time(d.RetryNotBefore).Format(time.RFC3339))
	}
	if d.Status == "sending" || d.Status == "pending" || d.Status == "retry_wait" {
		return errors.New("此通知正在发送或等待重试")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&d).Update("update_time", customtype.JsonTime(time.Now())).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.CertificateDelivery{}).Where("event_id = ? AND subscription_id = ? AND status IN ?", d.EventID, d.SubscriptionID, []string{"pending", "sending", "retry_wait"}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		var limited model.CertificateDelivery
		if err := tx.Where("event_id = ? AND channel_id = ?", d.EventID, d.ChannelID).Order("retry_not_before desc").First(&limited).Error; err != nil {
			return err
		}
		if time.Time(limited.RetryNotBefore).After(time.Now()) {
			return fmt.Errorf("渠道限流，最早重试时间：%s", time.Time(limited.RetryNotBefore).Format(time.RFC3339))
		}
		d.Id = uuid.GenUUID()
		d.Status, d.Error, d.Attempts = "pending", "", 0
		now := customtype.JsonTime(time.Now())
		d.CREATE_TIME, d.UPDATE_TIME, d.NextAttemptAt, d.LeaseUntil = now, now, now, customtype.JsonTime{}
		return tx.Create(&d).Error
	})
}
func CheckCertificateExpiryRisks(now time.Time) error {
	if !CertificateLifecycleAvailable() {
		return nil
	}
	var certs []model.SslConfig
	if err := global.GWAF_LOCAL_DB.Omit("KeyContent").Find(&certs).Error; err != nil {
		return err
	}
	for _, c := range certs {
		if time.Time(c.ValidTo).IsZero() {
			continue
		}
		days := global.CurrentSettings().GCONFIG_RECORD_SSLOrder_EXPIRE_DAY
		bindings, err := CertificateBindings(global.GWAF_LOCAL_DB, c.Id)
		if err != nil {
			return err
		}
		if len(bindings) == 0 {
			continue
		}
		effectiveExpiry := time.Time(c.ValidTo)
		for _, binding := range bindings {
			expires := time.Time(binding.ValidTo)
			if !expires.IsZero() && expires.Before(effectiveExpiry) {
				effectiveExpiry = expires
			}
		}
		if effectiveExpiry.Sub(now) > time.Duration(days)*24*time.Hour {
			continue
		}

		o := model.SslOrder{BaseOrm: c.BaseOrm, CertificateID: c.Id, ApplyDomain: c.Domains, ResultValidTo: customtype.JsonTime(effectiveExpiry)}
		o.Id = sslSubmissionHash([]string{"expiry", c.Id, now.UTC().Format("2006-01-02")})
		if err := global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
			return enqueueCertificateEvent(tx, o, "expiry", fmt.Sprintf("证书到期风险。域名：%s\n有效期至：%s\n请检查续期与部署状态。", c.Domains, effectiveExpiry.Format(time.RFC3339)), now)
		}); err != nil {
			return err
		}
	}
	return nil
}
