// Package config 提供配置和密钥管理
package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
)

const (
	// KeyDataFileName 统一密钥数据文件名（包含盐值 + verifier）
	KeyDataFileName = "key.dat"
	// LegacySaltFileName 旧版盐值文件名
	LegacySaltFileName = "salt.bin"
)

// KeyConfig 密钥配置
type KeyConfig struct {
	Salt        []byte // 16字节随机盐值
	DerivedKey  []byte // 派生的32字节AES密钥
	KeyFilePath string // 密钥文件路径
	IsLegacy    bool   // 是否来自旧版 salt.bin
}

// KeyFileData 统一密钥文件内容
type KeyFileData struct {
	Salt     []byte
	Verifier []byte
}

// GenerateSalt 生成随机盐值
func GenerateSalt(length int) ([]byte, error) {
	salt := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("生成盐值失败: %w", err)
	}
	return salt, nil
}

// DeriveKey 使用 Argon2id 从密码派生密钥
func DeriveKey(password string, salt []byte) []byte {
	// Argon2id 参数：time=3, memory=64MB, threads=4, keyLen=32
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
}

// SaveKeyData 保存统一密钥数据文件（salt + verifier）
func SaveKeyData(salt, verifier []byte, dir string) error {
	path := filepath.Join(dir, KeyDataFileName)
	data := make([]byte, 0, len(salt)+len(verifier))
	data = append(data, salt...)
	data = append(data, verifier...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("保存密钥文件失败: %w", err)
	}
	return nil
}

// LoadKeyData 加载统一密钥数据文件
func LoadKeyData(dir string) (*KeyFileData, error) {
	path := filepath.Join(dir, KeyDataFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 16 {
		return nil, errors.New("密钥文件损坏")
	}
	return &KeyFileData{
		Salt:     data[:16],
		Verifier: append([]byte(nil), data[16:]...),
	}, nil
}

// SaveSalt 保存盐值到旧版文件（兼容函数）
func SaveSalt(salt []byte, dir string) error {
	path := filepath.Join(dir, LegacySaltFileName)
	if err := os.WriteFile(path, salt, 0600); err != nil {
		return fmt.Errorf("保存盐值失败: %w", err)
	}
	return nil
}

// LoadSalt 从旧版文件加载盐值（兼容函数）
func LoadSalt(dir string) ([]byte, error) {
	return loadLegacySalt(dir)
}

// loadLegacySalt 从旧版文件加载盐值
func loadLegacySalt(dir string) ([]byte, error) {
	path := filepath.Join(dir, LegacySaltFileName)
	salt, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("未找到密钥文件，请先初始化")
		}
		return nil, fmt.Errorf("读取盐值失败: %w", err)
	}
	return salt, nil
}

// CheckInitialized 检查是否已初始化（key.dat 或旧版 salt.bin 是否存在）
func CheckInitialized(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, KeyDataFileName)); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, LegacySaltFileName)); err == nil {
		return true
	}
	return false
}

// Initialize 初始化密钥系统
// 首次使用时调用，生成盐值、verifier 并写入 key.dat
func Initialize(password string, dir string) (*KeyConfig, error) {
	if CheckInitialized(dir) {
		return nil, errors.New("已初始化，请勿重复初始化")
	}

	salt, err := GenerateSalt(16)
	if err != nil {
		return nil, err
	}

	key := DeriveKey(password, salt)
	verifierCipher, err := encryptWithKey(key, "PasswordBoxAuth")
	if err != nil {
		return nil, fmt.Errorf("生成验证器失败: %w", err)
	}

	if err := SaveKeyData(salt, []byte(verifierCipher), dir); err != nil {
		return nil, err
	}

	return &KeyConfig{
		Salt:        salt,
		DerivedKey:  key,
		KeyFilePath: filepath.Join(dir, KeyDataFileName),
		IsLegacy:    false,
	}, nil
}

// Unlock 使用主密码解锁，派生密钥
// 优先读取 key.dat 做确定性 verifier 校验；若不存在则降级读取 salt.bin
func Unlock(password string, dir string) (*KeyConfig, error) {
	// 尝试新版 key.dat
	keyFileData, err := LoadKeyData(dir)
	if err == nil {
		key := DeriveKey(password, keyFileData.Salt)
		plain, decryptErr := decryptWithKey(key, string(keyFileData.Verifier))
		if decryptErr != nil || plain != "PasswordBoxAuth" {
			return nil, errors.New("主密码错误")
		}
		return &KeyConfig{
			Salt:        keyFileData.Salt,
			DerivedKey:  key,
			KeyFilePath: filepath.Join(dir, KeyDataFileName),
			IsLegacy:    false,
		}, nil
	}

	// 降级旧版 salt.bin
	if os.IsNotExist(err) {
		salt, err := loadLegacySalt(dir)
		if err != nil {
			return nil, err
		}
		key := DeriveKey(password, salt)
		return &KeyConfig{
			Salt:        salt,
			DerivedKey:  key,
			KeyFilePath: filepath.Join(dir, LegacySaltFileName),
			IsLegacy:    true,
		}, nil
	}

	return nil, err
}

// GetKeyFromEnv 从环境变量获取密钥（开发调试用）
func GetKeyFromEnv() ([]byte, error) {
	keyStr := os.Getenv("PASSWORDBOX_KEY")
	if keyStr == "" {
		return nil, errors.New("环境变量 PASSWORDBOX_KEY 未设置")
	}

	// 支持 base64 编码的密钥
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		// 如果不是 base64，直接使用字符串
		key = []byte(keyStr)
	}

	// 确保密钥长度为 32 字节
	if len(key) != 32 {
		return nil, fmt.Errorf("密钥长度必须为 32 字节，当前 %d 字节", len(key))
	}

	return key, nil
}

// GenerateRandomKey 生成随机密钥（用于测试或特殊情况）
func GenerateRandomKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}
	return key, nil
}

// AES加密（内部使用）
func encryptWithKey(key []byte, plain string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	b := []byte(plain)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], b)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// AES解密（内部使用）
func decryptWithKey(key []byte, cryptoText string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("密文太短")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return string(ciphertext), nil
}
