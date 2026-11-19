// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafonekey

import (
	"HomeWaf/common/uuid"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"
)

var nginxWriteMu sync.Mutex
var listenHTTP = regexp.MustCompile(`(?m)^([ \t]*listen[ \t]+)80([ \t;])`)
var listenHTTPS = regexp.MustCompile(`(?m)^([ \t]*listen[ \t]+)443([ \t;])`)

// OneKeyModifyBt updates listen directives in the authorized panel directory.
func OneKeyModifyBt(btSavePath string) (error, string) {
	nginxWriteMu.Lock()
	defer nginxWriteMu.Unlock()
	root, err := openNginxDir(btSavePath)
	if err != nil {
		return err, ""
	}
	defer root.Close()
	files, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err, ""
	}
	count := 0
	for _, file := range files {
		if !file.Type().IsRegular() || filepath.Ext(file.Name()) != ".conf" {
			continue
		}
		before, mode, err := readNginxConfig(root, file.Name())
		if err != nil {
			return err, ""
		}
		after := listenHTTP.ReplaceAllString(string(before), "${1}81${2}")
		after = listenHTTPS.ReplaceAllString(after, "${1}444${2}")
		if after == string(before) {
			continue
		}
		record := model.OneKeyMod{
			BaseOrm: baseorm.BaseOrm{Id: uuid.GenUUID(), USER_CODE: global.GWAF_USER_CODE,
				Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: customtype.JsonTime(time.Now()), UPDATE_TIME: customtype.JsonTime(time.Now())},
			OpSystem: "宝塔", FilePath: filepath.Join(root.Name(), file.Name()),
			BeforeContent: string(before), AfterContent: after,
		}
		// Persist the recovery copy before changing a configuration on disk.
		if err := global.GWAF_LOCAL_LOG_DB.Create(&record).Error; err != nil {
			return err, ""
		}
		if err := replaceNginxConfig(root, file.Name(), after, mode); err != nil {
			return fmt.Errorf("修改失败，原始配置已保存在恢复记录中: %w", err), ""
		}
		count++
	}
	message := "修改文件数：" + strconv.Itoa(count)
	if count > 0 {
		message += " 请在宝塔面板上重启 Nginx"
	}
	return nil, message
}

func replaceNginxConfig(root *os.Root, name, content string, mode os.FileMode) error {
	tmp := ".homewaf-nginx-" + rand.Text()
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return err
	}
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return root.Rename(tmp, name)
}

// 读取 model.OneKeyMod 表中 IsRestore 为 0 的记录，并还原文件
func RestoreOneKeyMod(id string) error {
	nginxWriteMu.Lock()
	defer nginxWriteMu.Unlock()
	var oneKeyMod model.OneKeyMod
	err := global.GWAF_LOCAL_LOG_DB.Where("id = ?", id).First(&oneKeyMod).Error
	if err != nil {
		return err
	}
	if oneKeyMod.IsRestore == 1 {
		return errors.New("该记录已还原")
	}
	root, err := openNginxDir(filepath.Dir(oneKeyMod.FilePath))
	if err != nil {
		return err
	}
	defer root.Close()
	name := filepath.Base(oneKeyMod.FilePath)
	current, mode, err := readNginxConfig(root, name)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err != nil {
		mode = 0644
	}
	if string(current) != oneKeyMod.BeforeContent || err != nil {
		if err == nil && string(current) != oneKeyMod.AfterContent {
			return errors.New("配置已被另行修改，请核对恢复记录后手工处理")
		}
		if err := replaceNginxConfig(root, name, oneKeyMod.BeforeContent, mode); err != nil {
			return err
		}
	}
	return global.GWAF_LOCAL_LOG_DB.Model(&model.OneKeyMod{}).Where("id = ?", id).Update("is_restore", 1).Error
}
