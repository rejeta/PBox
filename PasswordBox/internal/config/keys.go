// Package config 提供配置和密钥管理
package config

import (
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
	// SaltFileName 盐值文件名
	SaltFileName = "salt.bin"
	// KeyFileName 密钥文件名（可选，用于存储环境变量方式的密钥）
	KeyFileName = "key.dat"
)

// KeyConfig 密钥配置
type KeyConfig struct {
	Salt        []byte // 16字节随机盐值
	DerivedKey  []byte // 派生的32字节AES密钥
	KeyFilePath string // 密钥文件路径
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

// SaveSalt 保存盐值到文件
func SaveSalt(salt []byte, dir string) error {
	path := filepath.Join(dir, SaltFileName)
	// 权限设置为仅所有者可读写 (0600)
	if err := os.WriteFile(path, salt, 0600); err != nil {
		return fmt.Errorf("保存盐值失败: %w", err)
	}
	return nil
}

// LoadSalt 从文件加载盐值
func LoadSalt(dir string) ([]byte, error) {
	path := filepath.Join(dir, SaltFileName)
	salt, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("未找到盐值文件，请先初始化")
		}
		return nil, fmt.Errorf("读取盐值失败: %w", err)
	}
	return salt, nil
}

// CheckInitialized 检查是否已初始化（盐值文件是否存在）
func CheckInitialized(dir string) bool {
	path := filepath.Join(dir, SaltFileName)
	_, err := os.Stat(path)
	return err == nil
}

// Initialize 初始化密钥系统
// 首次使用时调用，生成盐值并派生密钥
func Initialize(password string, dir string) (*KeyConfig, error) {
	// 检查是否已初始化
	if CheckInitialized(dir) {
		return nil, errors.New("已初始化，请勿重复初始化")
	}

	// 生成16字节随机盐值
	salt, err := GenerateSalt(16)
	if err != nil {
		return nil, err
	}

	// 保存盐值
	if err := SaveSalt(salt, dir); err != nil {
		return nil, err
	}

	// 派生密钥
	key := DeriveKey(password, salt)

	return &KeyConfig{
		Salt:        salt,
		DerivedKey:  key,
		KeyFilePath: filepath.Join(dir, SaltFileName),
	}, nil
}

// Unlock 使用主密码解锁，派生密钥
func Unlock(password string, dir string) (*KeyConfig, error) {
	// 加载盐值
	salt, err := LoadSalt(dir)
	if err != nil {
		return nil, err
	}

	// 派生密钥
	key := DeriveKey(password, salt)

	return &KeyConfig{
		Salt:        salt,
		DerivedKey:  key,
		KeyFilePath: filepath.Join(dir, SaltFileName),
	}, nil
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
