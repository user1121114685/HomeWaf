package wafsec

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
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
