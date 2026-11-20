package wafsec

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

func TestWindowsKeyDoesNotInheritEveryoneAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	if _, err := loadOrCreateDataKey(path); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("key DACL inherits permissions: %v", err)
	}
	wide, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := wide.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateDataKey(path); err == nil {
		t.Fatal("Everyone-readable key accepted")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsAdminPrivateFileAllowsAdministrators(t *testing.T) {
	path := filepath.Join(t.TempDir(), "initial_password.txt")
	if err := WriteAdminPrivateFile(path, []byte("fixture password")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "fixture password" {
		t.Fatalf("private file unreadable by its writer: %v", err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("administrator file DACL inherits permissions: %v", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("read administrator file DACL: %v", err)
	}
	hasAdministrators := false
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatal(err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
		if sid == "S-1-5-32-544" {
			hasAdministrators = true
		}
		if sid == "S-1-1-0" || sid == "S-1-5-32-545" {
			t.Fatalf("private file allows broad access for SID %s", sid)
		}
	}
	if !hasAdministrators {
		t.Fatal("private file does not grant access to the local Administrators group")
	}
}
