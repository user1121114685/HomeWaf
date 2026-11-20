// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafdb

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/platform/wafsec"
	"bufio"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"gorm.io/gorm"
)

// selectAccount 列出所有账号，优先默认选中 admin，让用户选择后返回账号名
func selectAccount() (string, bool) {
	var accounts []model.Account
	if err := global.GWAF_LOCAL_DB.Find(&accounts).Error; err != nil || len(accounts) == 0 {
		fmt.Println("No accounts found in database.")
		return "", false
	}

	defaultIndex := -1
	for i, a := range accounts {
		if a.LoginAccount == "admin" {
			defaultIndex = i
			break
		}
	}

	fmt.Println("Available accounts:")
	for i, a := range accounts {
		marker := ""
		if i == defaultIndex {
			marker = " [default]"
		}
		fmt.Printf("  [%d] %s%s\n", i+1, a.LoginAccount, marker)
	}

	reader := bufio.NewReader(os.Stdin)
	if defaultIndex >= 0 {
		fmt.Printf("Please enter account number (press Enter to use default '%s'): ", accounts[defaultIndex].LoginAccount)
	} else {
		fmt.Print("Please enter account number: ")
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" && defaultIndex >= 0 {
		return accounts[defaultIndex].LoginAccount, true
	}

	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(accounts) {
		fmt.Println("Invalid selection.")
		return "", false
	}
	return accounts[idx-1].LoginAccount, true
}

// ResetAdminPwd 重置密码
func ResetAdminPwd() {
	chosenAccount, ok := selectAccount()
	if !ok {
		return
	}

	randomPassword, gerr := utils.GenerateRandomPassword(12)
	if gerr != nil || randomPassword == "" {
		fmt.Printf("Failed to generate new password: %v\n", gerr)
		return
	}
	hash, herr := utils.BcryptHash(randomPassword)
	if herr != nil {
		fmt.Printf("Failed to hash new password: %v\n", herr)
		return
	}
	var receipt string
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		receipt = filepath.Join(utils.GetCurrentDir(), "data", "reset-password-"+rand.Text()+".txt")
		if err := wafsec.WritePrivateFile(receipt, []byte(randomPassword+"\n")); err != nil {
			fmt.Printf("Cannot save password to a private file: %v\n", err)
			return
		}
	}
	if err := resetAccountCredential(global.GWAF_LOCAL_DB, chosenAccount, hash); err != nil {
		if receipt != "" {
			_ = os.Remove(receipt)
		}
		fmt.Printf("Failed to reset password for '%s': %v\n", chosenAccount, err)
	} else if receipt != "" {
		fmt.Printf("Password reset. Read the new password from %s.\n", receipt)
	} else {
		fmt.Printf("Reset password for '%s' successfully, the new password is:\n", chosenAccount)
		fmt.Println(randomPassword)
		fmt.Println("Please keep it safe.")
	}
}

func resetAccountCredential(db *gorm.DB, account, hash string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		res := tx.Model(&model.Account{}).Where("login_account = ?", account).Updates(map[string]any{
			"LoginPassword": hash, "NeedChangePassword": 1, "PwdUpdateTime": now.Format("2006-01-02 15:04:05"),
			"UPDATE_TIME": customtype.JsonTime(now),
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("login_account = ?", account).Delete(&model.TokenInfo{}).Error
	})
}

// ResetAdminOTP 重置OTP
func ResetAdminOTP() {
	chosenAccount, ok := selectAccount()
	if !ok {
		return
	}

	global.GWAF_LOCAL_DB.Where("user_name = ?", chosenAccount).Delete(model.Otp{})
	fmt.Printf("Reset 2FA for '%s' successfully.\n", chosenAccount)
}
