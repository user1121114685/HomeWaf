package waf_service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// writeAppFile 在同目录替换文件；首次上传在系统初始权限上增加所有者执行位。
func writeAppFile(root *os.Root, destPath string, src io.Reader, mode *os.FileMode, expectedHash string) error {
	tempName := filepath.Join(filepath.Dir(destPath), ".samwaf-upload-"+rand.Text())
	perm := os.FileMode(0600)
	if mode == nil {
		perm = 0666
	}
	f, err := root.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer root.Remove(tempName)
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), src); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	actualHash := hex.EncodeToString(h.Sum(nil))
	if expectedHash != "" && !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("文件哈希校验失败，期望: %s，实际: %s", expectedHash, actualHash)
	}
	if mode == nil && runtime.GOOS != "windows" {
		info, err := f.Stat()
		if err != nil {
			return fmt.Errorf("读取临时文件权限失败: %w", err)
		}
		perm := info.Mode().Perm() | 0100
		mode = &perm
	}
	if mode != nil {
		if err := f.Chmod(mode.Perm()); err != nil {
			return fmt.Errorf("设置文件权限失败: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := root.Rename(tempName, destPath); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}
	return nil
}

func copyFile(root *os.Root, src, dst string) error {
	in, err := root.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("源文件不是普通文件")
	}
	mode := info.Mode().Perm()
	return writeAppFile(root, dst, in, &mode, "")
}
