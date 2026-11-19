// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/common/uuid"
	"HomeWaf/customtype"
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/baseorm"
	"HomeWaf/model/request"
	"HomeWaf/model/wafappmodel"
	"HomeWaf/wafapppath"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type WafAppService struct{}

var WafAppServiceApp = new(WafAppService)

// envBlacklist 禁止用户注入的环境变量键（可劫持进程/动态链接库行为）
var envBlacklist = map[string]struct{}{
	"LD_PRELOAD":            {},
	"LD_LIBRARY_PATH":       {},
	"LD_AUDIT":              {},
	"DYLD_INSERT_LIBRARIES": {},
	"DYLD_LIBRARY_PATH":     {},
	"LD_PRELOAD_ONCE":       {},
}

// openAppDir confines all file operations to one configured application root.
func openAppDir(code, dir string, create bool) (*os.Root, error) {
	p := wafapppath.Resolve(code, dir, global.GWAF_APP_ALLOW_DIRS)
	if !p.Valid {
		return nil, errors.New(p.Error)
	}
	rel, err := filepath.Rel(p.PathBase, p.ResolvedDir)
	if err != nil || !filepath.IsLocal(rel) {
		return nil, errors.New("应用目录不在允许范围内")
	}
	if create {
		if err := os.MkdirAll(p.PathBase, 0755); err != nil {
			return nil, err
		}
	}
	base, err := os.OpenRoot(p.PathBase)
	if err != nil || rel == "." {
		return base, err
	}
	defer base.Close()
	if create {
		if err := base.MkdirAll(rel, 0755); err != nil {
			return nil, err
		}
	}
	return base.OpenRoot(rel)
}

// validateEnv 校验用户提供的环境变量格式并拦截黑名单键
func validateEnv(env string) error {
	if env == "" {
		return nil
	}
	for _, part := range strings.Split(env, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eqIdx := strings.IndexByte(part, '=')
		if eqIdx <= 0 {
			return fmt.Errorf("环境变量格式错误（需为 KEY=VALUE）: %s", part)
		}
		key := strings.ToUpper(part[:eqIdx])
		if _, blocked := envBlacklist[key]; blocked {
			return fmt.Errorf("不允许设置环境变量 %s（存在安全风险）", part[:eqIdx])
		}
	}
	return nil
}

func (s *WafAppService) AddApi(req request.WafAppAddReq) (*model.WafApp, error) {
	if req.Name == "" {
		return nil, errors.New("应用名称不能为空")
	}
	if req.StartCmd == "" {
		return nil, errors.New("启动命令不能为空")
	}
	if err := validateEnv(req.Env); err != nil {
		return nil, err
	}

	bean := &model.WafApp{
		BaseOrm: baseorm.BaseOrm{
			Id:          uuid.GenUUID(),
			USER_CODE:   global.GWAF_USER_CODE,
			Tenant_ID:   global.GWAF_TENANT_ID,
			CREATE_TIME: customtype.JsonTime(time.Now()),
			UPDATE_TIME: customtype.JsonTime(time.Now()),
		},
		Code:            req.Code,
		Name:            req.Name,
		AppDir:          req.AppDir,
		StartCmd:        req.StartCmd,
		Env:             req.Env,
		AutoStart:       req.AutoStart,
		StartStatus:     req.StartStatus,
		StopMode:        req.StopMode,
		StopCmd:         req.StopCmd,
		StopTimeout:     req.StopTimeout,
		RestartPolicy:   req.RestartPolicy,
		RestartDelay:    req.RestartDelay,
		MaxRestartCount: req.MaxRestartCount,
		LogMaxLines:     req.LogMaxLines,
		Remarks:         req.Remarks,
	}
	if bean.Code == "" {
		bean.Code = bean.Id
	}
	if bean.StopMode == "" {
		bean.StopMode = "signal"
	}
	if bean.RestartPolicy == "" {
		bean.RestartPolicy = "no"
	}
	if bean.StopTimeout == 0 {
		bean.StopTimeout = 30
	}
	if bean.LogMaxLines == 0 {
		bean.LogMaxLines = 1000
	}

	root, err := openAppDir(bean.Code, bean.AppDir, true)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	bean.AppDir = root.Name()

	if err := global.GWAF_LOCAL_DB.Create(bean).Error; err != nil {
		return nil, err
	}
	return bean, nil
}

func (s *WafAppService) CheckIsExist(name string) int {
	var total int64
	global.GWAF_LOCAL_DB.Model(&model.WafApp{}).Where("name = ?", name).Count(&total)
	return int(total)
}

func (s *WafAppService) ModifyApi(req request.WafAppEditReq) error {
	var total int64
	global.GWAF_LOCAL_DB.Model(&model.WafApp{}).Where("name = ? AND id != ?", req.Name, req.Id).Count(&total)
	if total > 0 {
		return errors.New("应用名称已存在")
	}
	if err := validateEnv(req.Env); err != nil {
		return err
	}

	var existing model.WafApp
	if err := global.GWAF_LOCAL_DB.Where("id = ?", req.Id).First(&existing).Error; err != nil {
		return errors.New("应用不存在")
	}
	root, err := openAppDir(existing.Code, req.AppDir, true)
	if err != nil {
		return err
	}
	defer root.Close()

	updates := map[string]interface{}{
		"name":              req.Name,
		"app_dir":           root.Name(),
		"start_cmd":         req.StartCmd,
		"env":               req.Env,
		"auto_start":        req.AutoStart,
		"start_status":      req.StartStatus,
		"stop_mode":         req.StopMode,
		"stop_cmd":          req.StopCmd,
		"stop_timeout":      req.StopTimeout,
		"restart_policy":    req.RestartPolicy,
		"restart_delay":     req.RestartDelay,
		"max_restart_count": req.MaxRestartCount,
		"log_max_lines":     req.LogMaxLines,
		"remarks":           req.Remarks,
		"update_time":       customtype.JsonTime(time.Now()),
	}
	return global.GWAF_LOCAL_DB.Model(&model.WafApp{}).Where("id = ?", req.Id).Updates(updates).Error
}

func (s *WafAppService) GetDetailApi(req request.WafAppDetailReq) *model.WafApp {
	var bean model.WafApp
	global.GWAF_LOCAL_DB.Where("id = ?", req.Id).First(&bean)
	return &bean
}

func (s *WafAppService) GetDetailByCodeApi(code string) *model.WafApp {
	var bean model.WafApp
	global.GWAF_LOCAL_DB.Where("code = ?", code).First(&bean)
	return &bean
}

func (s *WafAppService) GetListApi(req request.WafAppSearchReq) ([]model.WafApp, int64) {
	var list []model.WafApp
	var total int64
	global.GWAF_LOCAL_DB.Model(&model.WafApp{}).Count(&total)
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	pageIndex := req.PageIndex
	if pageIndex <= 0 {
		pageIndex = 1
	}
	global.GWAF_LOCAL_DB.Limit(pageSize).Offset((pageIndex - 1) * pageSize).Find(&list)
	return list, total
}

func (s *WafAppService) storedAppRoot(code string, create bool) (*os.Root, error) {
	var app model.WafApp
	if err := global.GWAF_LOCAL_DB.Where("code = ?", code).First(&app).Error; err != nil {
		return nil, fmt.Errorf("读取应用 %s 失败: %w", code, err)
	}
	return openAppDir(app.Code, app.AppDir, create)
}

func validAppFilename(name string) bool {
	return name != "." && filepath.IsLocal(name) && !strings.ContainsAny(name, "/\\:\x00")
}

func (s *WafAppService) UploadFile(code, filename string, src io.Reader, expectedHash string) error {
	return s.installAppFile(code, filename, src, expectedHash, false)
}

func (s *WafAppService) UpgradeApp(code, filename string, src io.Reader, expectedHash string) error {
	return s.installAppFile(code, filename, src, expectedHash, true)
}

func (s *WafAppService) installAppFile(code, name string, src io.Reader, expectedHash string, backup bool) error {
	if !validAppFilename(name) {
		return errors.New("应用文件名无效")
	}
	root, err := s.storedAppRoot(code, true)
	if err != nil {
		return err
	}
	defer root.Close()
	var mode *os.FileMode
	if info, err := root.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("目标不是普通文件")
		}
		perm := info.Mode().Perm()
		mode = &perm
		if backup {
			dst := filepath.Join("backup", name+"."+time.Now().Format("20060102150405"))
			if err := root.MkdirAll("backup", 0755); err != nil {
				return fmt.Errorf("创建备份目录失败: %w", err)
			}
			if err := copyFile(root, name, dst); err != nil {
				return fmt.Errorf("备份应用失败: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeAppFile(root, name, src, mode, expectedHash)
}

func (s *WafAppService) RollbackApp(code, filename string) error {
	if !validAppFilename(filename) {
		return errors.New("备份文件名无效")
	}
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	if _, err := time.Parse("20060102150405", strings.TrimPrefix(ext, ".")); err != nil || !validAppFilename(name) {
		return errors.New("备份文件名必须包含有效的备份时间")
	}
	root, err := s.storedAppRoot(code, false)
	if err != nil {
		return err
	}
	defer root.Close()
	return copyFile(root, filepath.Join("backup", filename), name)
}

func (s *WafAppService) ListBackups(code string) ([]wafappmodel.BackupInfo, error) {
	root, err := s.storedAppRoot(code, false)
	if err != nil {
		if os.IsNotExist(err) {
			return []wafappmodel.BackupInfo{}, nil
		}
		return nil, err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), "backup")
	if err != nil {
		if os.IsNotExist(err) {
			return []wafappmodel.BackupInfo{}, nil
		}
		return nil, err
	}

	var result []wafappmodel.BackupInfo
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		info, err2 := entry.Info()
		if err2 != nil {
			continue
		}
		result = append(result, wafappmodel.BackupInfo{
			Filename:  entry.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	return result, nil
}
