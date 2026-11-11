// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package codetemplete

import (
	"HomeWaf/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeGeneration(t *testing.T) {
	source, err := filepath.Abs("tpl")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := os.CopyFS("tpl", os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	// 唯一校验定义字段信息的字符串
	fieldDefs := []string{
		"CaServerName:ca_server_name:string",
	}

	// 构造 `uniFields` 列表
	uniFields := []map[string]string{}
	for _, fieldDef := range fieldDefs {
		// 解析每个字段定义
		parts := strings.Split(fieldDef, ":")
		if len(parts) == 3 {
			field := map[string]string{
				"Name":     parts[0],
				"SqlField": parts[1],
				"SqlType":  parts[2],
			}
			uniFields = append(uniFields, field)
		}
	}

	fields := GetStructFields(model.CaServerInfo{})
	CodeGeneration("CaServerInfo", fields, uniFields)
	count := 0
	if err := filepath.WalkDir("output", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				t.Errorf("empty generated file: %s", path)
			}
			count++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if count != 7 {
		t.Fatalf("generated %d files, want 7", count)
	}
}
