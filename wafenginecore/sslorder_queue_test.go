package wafenginecore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/spec"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSSLOrderQueueRecoversUnnotifiedDurableWork(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "orders.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SslOrder{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldQueue := global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder
	global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = db, make(chan spec.ChanSslOrder)
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB, global.GWAF_CHAN_SSLOrder = oldDB, oldQueue
		conn, _ := db.DB()
		_ = conn.Close()
	})
	order := model.SslOrder{ApplyStatus: "submitted"}
	order.Id = "before-restart"
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	var processed string
	go func() {
		defer close(done)
		runSSLOrderQueue(ctx, func(_ int, o model.SslOrder) { processed = o.Id; cancel() })
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("durable pending order waited for lost queue notification")
	}
	if processed != order.Id {
		t.Fatal("scanner did not recover pending work")
	}
}
