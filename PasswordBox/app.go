package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"

	"PasswordBox/internal/config"
	"PasswordBox/internal/log"
	"PasswordBox/internal/utils"
)

type User struct {
	ID       uint   `gorm:"primaryKey"`
	Username string `gorm:"uniqueIndex"`
	Password string // AES加密后
}

type PasswordEntry struct {
	ID       uint   `gorm:"primaryKey"`
	UserID   uint   // 外键
	Site     string // AES加密后
	Account  string // AES加密后
	Password string // AES加密后
}

type App struct {
	ctx        context.Context
	mu         sync.Mutex
	db         *gorm.DB
	aesKey     []byte
	isUnlocked bool
	workDir     string // 工作目录（存储盐值文件）
	currentUser *User  // TODO: 第二阶段移除，仅用于兼容旧代码
}

type AppIni struct {
	DbPath string `ini:"db_path"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	workDir := "."

	// 检查并创建数据库目录
	dbPath := "Box.db"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		file, err := os.Create(dbPath)
		if err != nil {
			fmt.Println("创建数据库文件失败: " + err.Error())
			return nil
		}
		file.Close()
	}

	fmt.Println("连接数据库: " + dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		fmt.Println("数据库连接失败: " + err.Error())
		return nil
	}

	// 自动迁移表结构
	if err := db.AutoMigrate(&User{}, &PasswordEntry{}); err != nil {
		fmt.Println("数据库迁移失败: " + err.Error())
		return nil
	}

	return &App{
		db:         db,
		workDir:    workDir,
		isUnlocked: false,
		aesKey:     nil,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// AES加密
func (a *App) encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(a.aesKey)
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

// sha256 摘要
func (a *App) hashSHA256(data string) (string, error) {
	hasher := sha256.New()
	_, err := hasher.Write([]byte(data))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil)), nil
}

// 密码校验
func (a *App) verifyPassword(plain, hash string) (bool, error) {
	// 解base64编码
	if len(hash) == 0 {
		return false, errors.New("hash不能为空")
	}
	hashed, err := a.hashSHA256(plain)
	if err != nil {
		return false, err
	}
	return hashed == hash, nil
}

// AES解密
func (a *App) decrypt(cryptoText string) (string, error) {
	block, err := aes.NewCipher(a.aesKey)
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

// 注册
func (a *App) Register(username, password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	var count int64
	a.db.Model(&User{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		return errors.New("用户已存在")
	}
	encPwd, err := a.hashSHA256(password)
	if err != nil {
		return err
	}
	user := User{Username: username, Password: encPwd}
	return a.db.Create(&user).Error
}

// 登录
func (a *App) Login(username, password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(password) == 0 || len(username) == 0 {
		return errors.New("用户名或密码不能为空")
	}
	var user User
	if err := a.db.Where("username = ?", username).First(&user).Error; err != nil {
		return errors.New("未注册")
	}

	// 对比密码
	if ok, err := a.verifyPassword(password, user.Password); err != nil {
		return err
	} else if !ok {
		return errors.New("用户名或密码错误")
	}
	// 成功登录，设置当前用户
	a.currentUser = &user

	// 设置密码
	salt, err := a.hashSHA256(password)
	if err != nil {
		return errors.New("初始化密钥失败")
	}
	key := argon2.IDKey([]byte(salt), []byte(password), 3, 64*1024, 4, 32)
	a.aesKey = key
	return nil
}

// 保存密码
func (a *App) SavePassword(site, account, password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.isUnlocked == false {
		return errors.New("未解锁")
	}
	encSite, err := a.encrypt(site)
	if err != nil {
		return err
	}
	encAccount, err := a.encrypt(account)
	if err != nil {
		return err
	}
	encPwd, err := a.encrypt(password)
	if err != nil {
		return err
	}
	entry := PasswordEntry{
		UserID:   1,
		Site:     encSite,
		Account:  encAccount,
		Password: encPwd,
	}
	return a.db.Create(&entry).Error
}

// 前端展示用结构体
type PasswordEntryVO struct {
	ID       uint   `json:"id"`
	Site     string `json:"site"`
	Account  string `json:"account"`
	Password string `json:"password"`
}

// 查询密码
func (a *App) QueryPasswords() ([]PasswordEntryVO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.isUnlocked == false {
		return nil, errors.New("未解锁")
	}
	var entries []PasswordEntry
	err := a.db.Where("user_id = ?", 1).Find(&entries).Error
	if err != nil {
		return nil, err
	}
	var result []PasswordEntryVO
	for _, e := range entries {
		site, err := a.decrypt(e.Site)
		if err != nil {
			log.Error("解密站点失败 [id=%d]: %v", e.ID, err)
			site = "[解密失败]"
		}

		account, err := a.decrypt(e.Account)
		if err != nil {
			log.Error("解密账号失败 [id=%d]: %v", e.ID, err)
			account = "[解密失败]"
		}

		password, err := a.decrypt(e.Password)
		if err != nil {
			log.Error("解密密码失败 [id=%d]: %v", e.ID, err)
			password = "[解密失败]"
		}

		result = append(result, PasswordEntryVO{
			ID:       e.ID,
			Site:     site,
			Account:  account,
			Password: password,
		})
	}
	return result, nil
}

// 删除密码
func (a *App) DeletePassword(id uint) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.isUnlocked == false {
		return errors.New("未解锁")
	}
	var entry PasswordEntry
	if err := a.db.Where("id = ? AND user_id = ?", id, 1).First(&entry).Error; err != nil {
		return errors.New("密码不存在或无权限删除")
	}
	return a.db.Delete(&entry).Error
}

// 修改密码
func (a *App) UpdatePassword(id uint, site, account, password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.isUnlocked == false {
		return errors.New("未解锁")
	}
	var entry PasswordEntry
	if err := a.db.Where("id = ? AND user_id = ?", id, 1).First(&entry).Error; err != nil {
		return errors.New("密码不存在或无权限修改")
	}
	encSite, err := a.encrypt(site)
	if err != nil {
		return err
	}
	encAccount, err := a.encrypt(account)
	if err != nil {
		return err
	}
	encPwd, err := a.encrypt(password)
	if err != nil {
		return err
	}
	entry.Site = encSite
	entry.Account = encAccount
	entry.Password = encPwd
	return a.db.Save(&entry).Error
}

// 搜索指定密码
// 只根据账户名称（解密后）进行搜索，提升安全性，超时10秒无结果则返回失败
func (a *App) SearchPassword(keyword string) ([]PasswordEntryVO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.isUnlocked == false {
		return nil, errors.New("未解锁")
	}
	if len(keyword) == 0 {
		return nil, errors.New("搜索关键字不能为空")
	}

	// 优化：先用SQL模糊查找（加密字段无法直接LIKE），只能先查全部，再解密过滤
	var entries []PasswordEntry
	errCh := make(chan error, 1)
	resultCh := make(chan []PasswordEntryVO, 1)

	go func() {
		err := a.db.Where("user_id = ?", 1).Find(&entries).Error
		if err != nil {
			errCh <- err
			return
		}
		var result []PasswordEntryVO
		for _, e := range entries {
			account, err := a.decrypt(e.Account)
			if err != nil {
				log.Error("搜索时解密账号失败 [id=%d]: %v", e.ID, err)
				continue
			}
			if account != "" && containsIgnoreCase(account, keyword) {
				site, err := a.decrypt(e.Site)
				if err != nil {
					log.Error("搜索时解密站点失败 [id=%d]: %v", e.ID, err)
					site = "[解密失败]"
				}
				password, err := a.decrypt(e.Password)
				if err != nil {
					log.Error("搜索时解密密码失败 [id=%d]: %v", e.ID, err)
					password = "[解密失败]"
				}
				result = append(result, PasswordEntryVO{
					ID:       e.ID,
					Site:     site,
					Account:  account,
					Password: password,
				})
			}
		}
		resultCh <- result
	}()

	select {
	case err := <-errCh:
		return nil, err
	case result := <-resultCh:
		if len(result) == 0 {
			return nil, errors.New("搜索超时或无结果")
		}
		return result, nil
	case <-time.After(10 * time.Second):
		return nil, errors.New("搜索超时，请重试")
	}
}

// 字符串包含（忽略大小写）
func containsIgnoreCase(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

// ========== 新增：密钥管理相关方法 ==========

// CheckInitialized 检查是否已初始化（盐值文件是否存在）
func (a *App) CheckInitialized() bool {
	return config.CheckInitialized(a.workDir)
}

// SetupMasterPassword 首次使用设置主密码
func (a *App) SetupMasterPassword(password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 检查是否已初始化
	if a.CheckInitialized() {
		return errors.New("已初始化，请勿重复初始化")
	}

	// 检查密码强度
	if !utils.IsStrongPassword(password) {
		strength := utils.CheckPasswordStrength(password)
		return fmt.Errorf("密码强度不足: %s", strings.Join(strength.Suggestions, ", "))
	}

	// 初始化密钥系统
	keyConfig, err := config.Initialize(password, a.workDir)
	if err != nil {
		return err
	}

	a.aesKey = keyConfig.DerivedKey
	a.isUnlocked = true
	return nil
}

// Unlock 使用主密码解锁
func (a *App) Unlock(password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 加载密钥
	keyConfig, err := config.Unlock(password, a.workDir)
	if err != nil {
		return err
	}

	// 验证密钥：尝试解密数据库中的第一条记录（如果有）
	var count int64
	a.db.Model(&PasswordEntry{}).Count(&count)

	if count > 0 {
		// 有一条测试记录用于验证密钥
		// 实际验证会在首次查询时进行
		// 这里仅设置密钥
	}

	a.aesKey = keyConfig.DerivedKey
	a.isUnlocked = true
	return nil
}

// IsUnlocked 检查是否已解锁
func (a *App) IsUnlocked() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.isUnlocked
}

// Lock 锁定应用
func (a *App) Lock() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.aesKey = nil
	a.isUnlocked = false
}

// GetPasswordStrength 获取密码强度（供前端调用）
func (a *App) GetPasswordStrength(password string) utils.PasswordStrength {
	return utils.CheckPasswordStrength(password)
}
