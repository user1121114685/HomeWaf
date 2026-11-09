package wafsec

import (
	"errors"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows ignores Unix 0600. Give only the service account and SYSTEM access,
// with inheritance disabled, before writing any secret bytes.
func restrictKeyFile(f *os.File) error {
	path, err := windows.UTF16PtrFromString(f.Name())
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(path, windows.WRITE_DAC|windows.READ_CONTROL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;SY)(A;;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

func checkKeyFile(f *os.File, _ os.FileInfo) error {
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	defer runtime.KeepAlive(sd)
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return errors.New("key file must have a restrictive Windows DACL")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	allowed := map[string]bool{user.User.Sid.String(): true, "S-1-5-18": true, "S-1-5-32-544": true}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return errors.New("unsupported key file access rule")
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Mask != 0 && !allowed[sid.String()] {
			return errors.New("key file grants access beyond its service account, SYSTEM and administrators")
		}
	}
	return nil
}
