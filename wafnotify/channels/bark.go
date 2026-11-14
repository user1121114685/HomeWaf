package channels

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func validateBarkEncryption(c Config) error {
	algorithm := c.String("transformation")
	if algorithm == "" || algorithm == "none" {
		return nil
	}
	length := 16
	if strings.HasPrefix(algorithm, "AES192") {
		length = 24
	}
	if strings.HasPrefix(algorithm, "AES256") {
		length = 32
	}
	if len([]byte(c.String("key"))) != length {
		return fmt.Errorf("Bark 加密密钥须为 %d 字节", length)
	}
	iv := c.String("iv")
	expected := 16
	if strings.Contains(algorithm, "GCM") {
		expected = 12
	}
	if !strings.Contains(algorithm, "ECB") && iv != "" && len([]byte(iv)) != expected {
		return fmt.Errorf("Bark IV 须为 %d 字节", expected)
	}
	return nil
}

func barkPayload(c Config, payload Config) (interface{}, error) {
	algorithm := c.String("transformation")
	if algorithm == "" || algorithm == "none" {
		return payload, nil
	}
	if err := validateBarkEncryption(c); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(c.String("key")))
	if err != nil {
		return nil, fmt.Errorf("Bark 加密密钥无效")
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	iv := []byte(c.String("iv"))
	isECB := strings.Contains(algorithm, "ECB")
	isGCM := strings.Contains(algorithm, "GCM")
	if !isECB && len(iv) == 0 {
		length := 16
		if isGCM {
			length = 12
		}
		iv = make([]byte, length)
		const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		for i := range iv {
			var b [1]byte
			for {
				if _, err = rand.Read(b[:]); err != nil {
					return nil, err
				}
				if int(b[0]) < 248 {
					iv[i] = alphabet[int(b[0])%len(alphabet)]
					break
				}
			}
		}
	}
	var encrypted []byte
	if isGCM {
		gcm, e := cipher.NewGCM(block)
		if e != nil {
			return nil, e
		}
		encrypted = gcm.Seal(nil, iv, plain, nil)
	} else {
		padding := aes.BlockSize - len(plain)%aes.BlockSize
		for i := 0; i < padding; i++ {
			plain = append(plain, byte(padding))
		}
		encrypted = make([]byte, len(plain))
		if isECB {
			for i := 0; i < len(plain); i += aes.BlockSize {
				block.Encrypt(encrypted[i:i+aes.BlockSize], plain[i:i+aes.BlockSize])
			}
		} else {
			cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, plain)
		}
	}
	form := url.Values{"ciphertext": {base64.StdEncoding.EncodeToString(encrypted)}}
	if !isECB {
		form.Set("iv", string(iv))
	}
	return form, nil
}
