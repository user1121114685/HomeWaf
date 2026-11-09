package wafsec

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// 每实例静态数据加密密钥（DEK）。与通讯密钥 GWAF_COMMUNICATION_KEY 彻底分离：
// 通讯密钥全实例共用且随源码公开，只适合做传输层混淆；落库的敏感字段（Access 的
// TOTP/rq 签名密钥、CDN 厂商凭证、管理端 2FA 密钥等）改用本模块的每实例随机密钥，
// 拿到数据库文件不再等于拿到明文。
//
// 数据密钥放在安装目录的 .homewaf-keys/，与 data/ 数据库目录分开。
// Unix 权限 0600，Windows 限定 DACL；不进数据库、不经 API 回显。运营方可用
// security.data_key_file 指定自管位置。
//
// 密文格式：DataEncrypt 产出带 "swk1:" 前缀（AES-256-GCM）；运行期拒绝公开
// 通讯密钥加密的旧 CBC 数据，防止攻击者伪造旧格式凭证来绕过完整性校验。

const (
	// DataKeyPrefixV1 是 swk1 版密文前缀：AES-256-GCM。
	DataKeyPrefixV1 = "swk1:"
	dataKeyDirName  = ".keys"
	dataKeyFileName = "data_key"
	dataKeyLen      = 32 // AES-256
)

var (
	dataKey     []byte
	dataKeyOnce sync.Once
	dataKeyErr  error
	dataKeyMu   sync.RWMutex
)

// InitDataKey 加载（首次启动则生成）每实例 DEK。dataDir 为安装目录（GetCurrentDir），
// customFile 为 config.yml 中 security.data_key_file 声明的自管路径（可空）。
// 该函数应在数据库初始化之前调用，且全进程只需成功一次。
func InitDataKey(dataDir, customFile string) error {
	dataKeyOnce.Do(func() {
		path := strings.TrimSpace(customFile)
		if path == "" {
			path = filepath.Join(dataDir, ".homewaf-keys", dataKeyFileName)
		}
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			if strings.TrimSpace(customFile) != "" {
				dataKeyErr = errors.New("configured data key is missing; restore or provision the key file")
				return
			}
			if _, oldErr := os.Lstat(filepath.Join(dataDir, "data", dataKeyDirName, dataKeyFileName)); !os.IsNotExist(oldErr) {
				dataKeyErr = errors.New("legacy data key found; move it to .homewaf-keys/data_key or set security.data_key_file")
				return
			}
			files, globErr := filepath.Glob(filepath.Join(dataDir, "data", "*.db"))
			if globErr != nil || len(files) != 0 {
				dataKeyErr = errors.New("database exists but data key is missing; restore the original key instead of generating a replacement")
				return
			}
		}
		key, err := loadOrCreateDataKey(path)
		if err != nil {
			dataKeyErr = err
			return
		}
		dataKeyMu.Lock()
		dataKey = key
		dataKeyMu.Unlock()
	})
	return dataKeyErr
}

// DataKeyReady 返回 DEK 是否已成功加载。
func DataKeyReady() bool {
	dataKeyMu.RLock()
	defer dataKeyMu.RUnlock()
	return len(dataKey) == dataKeyLen
}

func loadOrCreateDataKey(path string) ([]byte, error) {
	return loadOrCreateKey(path, dataKeyLen)
}

// currentDataKey 取当前 DEK 的副本；未就绪返回错误。
func currentDataKey() ([]byte, error) {
	dataKeyMu.RLock()
	defer dataKeyMu.RUnlock()
	if len(dataKey) != dataKeyLen {
		return nil, errors.New("数据密钥尚未初始化")
	}
	k := make([]byte, dataKeyLen)
	copy(k, dataKey)
	return k, nil
}

// DataEncrypt 用每实例 DEK 加密并返回带 swk1: 前缀的密文（AES-256-GCM）。空串原样返回。
func DataEncrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	key, err := currentDataKey()
	if err != nil {
		return "", err
	}
	defer clear(key)
	ct, err := gcmEncrypt([]byte(plain), key)
	if err != nil {
		return "", err
	}
	return DataKeyPrefixV1 + ct, nil
}

// DataDecrypt accepts authenticated instance ciphertext only. The second
// argument is retained for source compatibility; it cannot enable legacy CBC.
func DataDecrypt(enc string, _ []byte) (string, error) {
	if enc == "" {
		return "", nil
	}
	if strings.HasPrefix(enc, DataKeyPrefixV1) {
		key, err := currentDataKey()
		if err != nil {
			return "", err
		}
		defer clear(key)
		b, err := gcmDecrypt(strings.TrimPrefix(enc, DataKeyPrefixV1), key)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", errors.New("unauthenticated legacy data ciphertext is disabled")
}

// IsDataKeyCiphertext 判断是否为 swk1 新格式密文。
func IsDataKeyCiphertext(enc string) bool {
	return strings.HasPrefix(enc, DataKeyPrefixV1)
}

// gcmEncrypt: 输出 base64(nonce ‖ ciphertext ‖ tag)。
func gcmEncrypt(plain, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func gcmDecrypt(b64 string, key []byte) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}
