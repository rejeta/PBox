package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
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
	"gopkg.in/ini.v1"
	"gorm.io/gorm"
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
	ctx         context.Context
	currentUser *User
	mu          sync.Mutex
	db          *gorm.DB
	aesKey      []byte
}

type AppIni struct {
	DbPath string `ini:"db_path"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	// AES密钥长度必须为16/24/32字节
	key := []byte("yuDuCM3zjYZDM1675SFnAzxacud8V7H5") // 32字节示例，生产环境请安全生成

	// 打开配置文件
	var appInfo AppIni
	cfg, err := ini.Load("PasswordBox.ini")
	if err != nil {
		fmt.Println("配置文件加载失败: " + err.Error())
		return nil
	}

	if err := cfg.MapTo(&appInfo); err != nil {
		fmt.Println("配置文件解析失败: " + err.Error())
		return nil
	}

	// 如果配置文件中没有设置数据库路径，则使用默认路径
	if appInfo.DbPath == "" {
		appInfo.DbPath = "Box.db" // 默认数据库路径
	}
	// 打开数据库连接
	if appInfo.DbPath == "" {
		fmt.Println("数据库路径未设置")
		return nil
	}
	if !strings.HasSuffix(appInfo.DbPath, ".db") {
		fmt.Println("数据库路径必须以 .db 结尾")
		return nil
	}
	// 打开数据库
	if _, err := os.Stat(appInfo.DbPath); os.IsNotExist(err) {
		// 如果数据库文件不存在，创建一个新的
		file, err := os.Create(appInfo.DbPath)
		if err != nil {
			fmt.Println("创建数据库文件失败: " + err.Error())
			return nil
		}
		file.Close()
	}
	// 连接数据库
	if appInfo.DbPath == "" {
		fmt.Println("数据库路径不能为空")
		return nil
	}

	fmt.Println("连接数据库: " + appInfo.DbPath)
	db, err := gorm.Open(sqlite.Open(appInfo.DbPath), &gorm.Config{})
	if err != nil {
		fmt.Println("数据库连接失败: " + err.Error())
		return nil
	}
	// 自动迁移表结构
	db.AutoMigrate(&User{}, &PasswordEntry{})
	return &App{db: db, aesKey: key}
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
	if a.currentUser == nil {
		return errors.New("未登录")
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
		UserID:   a.currentUser.ID,
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
	if a.currentUser == nil {
		return nil, errors.New("未登录")
	}
	var entries []PasswordEntry
	err := a.db.Where("user_id = ?", a.currentUser.ID).Find(&entries).Error
	if err != nil {
		return nil, err
	}
	var result []PasswordEntryVO
	for _, e := range entries {
		site, _ := a.decrypt(e.Site)
		account, _ := a.decrypt(e.Account)
		password, _ := a.decrypt(e.Password)
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
	if a.currentUser == nil {
		return errors.New("未登录")
	}
	var entry PasswordEntry
	if err := a.db.Where("id = ? AND user_id = ?", id, a.currentUser.ID).First(&entry).Error; err != nil {
		return errors.New("密码不存在或无权限删除")
	}
	return a.db.Delete(&entry).Error
}

// 修改密码
func (a *App) UpdatePassword(id uint, site, account, password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentUser == nil {
		return errors.New("未登录")
	}
	var entry PasswordEntry
	if err := a.db.Where("id = ? AND user_id = ?", id, a.currentUser.ID).First(&entry).Error; err != nil {
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
	if a.currentUser == nil {
		return nil, errors.New("未登录")
	}
	if len(keyword) == 0 {
		return nil, errors.New("搜索关键字不能为空")
	}

	// 优化：先用SQL模糊查找（加密字段无法直接LIKE），只能先查全部，再解密过滤
	var entries []PasswordEntry
	errCh := make(chan error, 1)
	resultCh := make(chan []PasswordEntryVO, 1)

	go func() {
		err := a.db.Where("user_id = ?", a.currentUser.ID).Find(&entries).Error
		if err != nil {
			errCh <- err
			return
		}
		var result []PasswordEntryVO
		for _, e := range entries {
			account, _ := a.decrypt(e.Account)
			if account != "" && containsIgnoreCase(account, keyword) {
				site, _ := a.decrypt(e.Site)
				password, _ := a.decrypt(e.Password)
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
