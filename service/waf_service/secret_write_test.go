package waf_service

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/request"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSecretServicesFailWithoutInstanceKey(t *testing.T) {
	if os.Getenv("HOMEWAF_TEST_NO_SECRET_KEY") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSecretServicesFailWithoutInstanceKey$")
		cmd.Env = append(os.Environ(), "HOMEWAF_TEST_NO_SECRET_KEY=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("missing-key service test: %v\n%s", err, output)
		}
		return
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "secrets.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := db.DB()
	defer raw.Close()
	global.GWAF_LOCAL_DB = db
	if err := db.AutoMigrate(&model.SystemConfig{}, &model.Otp{}, &model.PrivateInfo{}); err != nil {
		t.Fatal(err)
	}
	if err := WafSystemConfigServiceApp.AddApi(request.WafSystemConfigAddReq{Item: "debug_pwd", Value: "secret"}); err == nil {
		t.Fatal("system secret write succeeded without encryption")
	}
	if err := WafOtpServiceApp.BindApi(request.WafOtpBindReq{UserName: "fixture", Secret: "JBSWY3DPEHPK3PXP"}); err == nil {
		t.Fatal("OTP write succeeded without encryption")
	}
	if err := WafPrivateInfoServiceApp.AddApi(request.WafPrivateInfoAddReq{PrivateKey: "fixture", PrivateValue: "secret"}); err == nil {
		t.Fatal("private secret write succeeded without encryption")
	}
	for _, table := range []string{"system_configs", "otps", "private_infos"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("failed write left data in %s: %v", table, err)
		}
	}
}
