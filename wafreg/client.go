// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafreg

import (
	"HomeWaf/global"
	"HomeWaf/internal/licensing"
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafsec"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var saveMu sync.Mutex

func licensePath() string { return filepath.Join(utils.GetCurrentDir(), "registration_data.bin") }

func GenClientMachineInfo() model.MachineInfo {
	info := model.MachineInfo{Version: "v2", ClientServerName: global.GWAF_CUSTOM_SERVER_NAME, ClientTenantId: global.GWAF_TENANT_ID, ClientUserCode: global.GWAF_USER_CODE}
	installationID, err := wafsec.InstallationID()
	if err == nil && info.ClientUserCode != "" {
		identity, _ := json.Marshal([]string{licensing.Product, installationID, info.ClientTenantId, info.ClientUserCode})
		info.MachineID = fmt.Sprintf("%x", sha256.Sum256(identity))
	}
	return info
}

// Legacy RSA requests are deliberately unsupported: HomeWaf uses signed v2 licenses.
func GenClientMachineInfoWithEncrypt() (string, error) {
	return "", errors.New("legacy registration requests are unsupported; use the v2 instance ID")
}
func verify(document []byte) (licensing.Claims, error) {
	keys, err := licensing.ParseKeyring(licensing.TrustedKeysJSON)
	if err != nil {
		return licensing.Claims{}, fmt.Errorf("invalid release trust store: %w", err)
	}
	return licensing.Verify(document, keys, GenClientMachineInfo().MachineID, time.Now())
}
func infoFor(c licensing.Claims) model.RegistrationInfo {
	return model.RegistrationInfo{Version: "v2", Username: c.Subject, MemberType: c.Edition, MachineID: c.InstanceID, ExpiryDate: c.ExpiresAt, IsExpiry: !time.Now().Before(c.ExpiresAt), LicenseID: c.LicenseID, Features: append([]string{}, c.Features...)}
}
func VerifyServerReg(document []byte) (bool, model.RegistrationInfo, error) {
	c, err := verify(document)
	if err != nil {
		return false, model.RegistrationInfo{}, err
	}
	return true, infoFor(c), nil
}
func VerifyServerRegByDefaultFile() (bool, model.RegistrationInfo, error) {
	c, err := verifiedFileClaims()
	if err != nil {
		return false, model.RegistrationInfo{}, err
	}
	return true, infoFor(c), nil
}

// Every entitlement decision authenticates the signed file. Database flags, UI
// state and a previously cached license cannot keep a tampered grant alive.
func verifiedFileClaims() (licensing.Claims, error) {
	path := licensePath()
	info, err := os.Lstat(path)
	if err != nil {
		return licensing.Claims{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > licensing.MaxDocumentSize {
		return licensing.Claims{}, errors.New("invalid license file")
	}
	f, err := os.Open(path)
	if err != nil {
		return licensing.Claims{}, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return licensing.Claims{}, errors.New("license file changed while opening")
	}
	document, err := io.ReadAll(io.LimitReader(f, licensing.MaxDocumentSize+1))
	if err != nil {
		return licensing.Claims{}, err
	}
	return verify(document)
}
func CurrentInfo() model.RegistrationInfo {
	c, err := verifiedFileClaims()
	if err != nil {
		return model.RegistrationInfo{Version: "v2", MemberType: "noncommercial", MachineID: GenClientMachineInfo().MachineID}
	}
	return infoFor(c)
}
func HasFeature(feature string) bool {
	c, err := verifiedFileClaims()
	return err == nil && c.Allows(feature, time.Now())
}
func Save(document []byte) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	if _, err := verify(document); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(licensePath()), ".homewaf-license-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(document); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, licensePath()); err != nil {
		return err
	}
	_, _, err = VerifyServerRegByDefaultFile()
	return err
}
func CheckExpiry(date time.Time) (int, bool) {
	remaining := time.Until(date)
	if remaining <= 0 {
		return 0, true
	}
	return int((remaining + 24*time.Hour - 1) / (24 * time.Hour)), false
}
