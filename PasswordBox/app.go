package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"PasswordBox/internal/config"
	"PasswordBox/internal/log"
	"PasswordBox/internal/utils"
)


type PasswordEntry struct { // 单用户模式密码条目
	ID       uint   `gorm:"primaryKey"`
	Title      string // 条目名称（加密）
	URL        string // 网站地址（加密）
	Username   string // 用户名（加密）
	Password   string // 密码（加密）
	Note       string // 备注（加密）
	IsFavorite bool   // 是否收藏（不加密）
}

type App struct {
	ctx        context.Context
	mu         sync.Mutex
	db         *gorm.DB
	aesKey     []byte
	isUnlocked bool
	workDir     string // 工作目录（存储盐值文件）
}

type AppIni struct {
	DbPath string `ini:"db_path"`
}

// NewApp creates a new App application struct (default paths)
func NewApp() *App {
	return NewAppWithPath(".", "Box.db")
}

// NewAppWithPath creates a new App with configurable paths (for testing)
func NewAppWithPath(workDir, dbPath string) *App {
	if err := os.MkdirAll(workDir, 0755); err != nil {
		log.Error("创建工作目录失败: " + err.Error())
		return nil
	}

	fullDbPath := filepath.Join(workDir, dbPath)
	if _, err := os.Stat(fullDbPath); os.IsNotExist(err) {
		file, err := os.Create(fullDbPath)
		if err != nil {
			log.Error("创建数据库文件失败: " + err.Error())
			return nil
		}
		file.Close()
	}

	log.Info("连接数据库: %s", fullDbPath)
	db, err := gorm.Open(sqlite.Open(fullDbPath), &gorm.Config{})
	if err != nil {
		log.Error("数据库连接失败: " + err.Error())
		return nil
	}

	if err := db.AutoMigrate(&PasswordEntry{}); err != nil {
		log.Error("数据库迁移失败: " + err.Error())
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
	log.Info("应用启动完成")
}

// shutdown is called when the app exits
func (a *App) shutdown(ctx context.Context) {
	log.Info("应用正在关闭")
	a.Close()
	log.Close()
}

// domReady is called when the frontend has loaded
func (a *App) domReady(ctx context.Context) {
	log.Info("前端界面加载完成")
}

// Close 关闭数据库连接
func (a *App) Close() error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
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

// Register 注册用户（单用户模式，仅保留兼容性）
func (a *App) Register(username, password string) error {
	return errors.New("单用户模式，请使用初始化功能")
}

// Login 用户登录（单用户模式，仅保留兼容性）
func (a *App) Login(username, password string) error {
	return a.Unlock(password)
}

// 保存密码
func (a *App) SavePassword(title, url, username, password, note string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isUnlocked {
		return errors.New("未解锁")
	}
	encTitle, err := a.encrypt(title)
	if err != nil {
		return err
	}
	encURL, err := a.encrypt(url)
	if err != nil {
		return err
	}
	encUsername, err := a.encrypt(username)
	if err != nil {
		return err
	}
	encPassword, err := a.encrypt(password)
	if err != nil {
		return err
	}
	encNote, err := a.encrypt(note)
	if err != nil {
		return err
	}
	entry := PasswordEntry{
		Title:    encTitle,
		URL:      encURL,
		Username: encUsername,
		Password: encPassword,
		Note:     encNote,
	}
	if err := a.db.Create(&entry).Error; err != nil {
		return err
	}
	log.Info("保存密码条目 [id=%d]", entry.ID)
	return nil
}

// 前端展示用结构体
type EntryVO struct {
	ID         uint   `json:"id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Note       string `json:"note"`
	IsFavorite bool   `json:"isFavorite"`
}

// 查询密码，支持筛选和排序
// filter: all | favorite | recent
// sortBy: title | created | updated
func (a *App) QueryPasswords(filter, sortBy string) ([]EntryVO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isUnlocked {
		return nil, errors.New("未解锁")
	}
	var entries []PasswordEntry
	query := a.db.Model(&PasswordEntry{})

	// 筛选
	switch filter {
	case "favorite":
		query = query.Where("is_favorite = ?", true)
	case "recent":
		// 使用 ID 降序模拟最近添加（GORM 默认自增主键）
		query = query.Where("id > (SELECT MAX(id) - 10 FROM password_entries)")
	}

	// 排序
	switch sortBy {
	case "title":
		query = query.Order("id") // 加密字段无法直接按title排序，用id作为稳定顺序
	case "created":
		query = query.Order("id desc")
	case "updated":
		query = query.Order("id desc") // 当前无updated_at字段，用id desc近似
	default:
		query = query.Order("id desc")
	}
	err := query.Find(&entries).Error
	if err != nil {
		return nil, err
	}
	var result []EntryVO
	for _, e := range entries {
		title, err := a.decrypt(e.Title)
		if err != nil {
			log.Error("解密标题失败 [id=%d]: %v", e.ID, err)
			title = "[解密失败]"
		}

		url, err := a.decrypt(e.URL)
		if err != nil {
			log.Error("解密URL失败 [id=%d]: %v", e.ID, err)
			url = "[解密失败]"
		}

		username, err := a.decrypt(e.Username)
		if err != nil {
			log.Error("解密用户名失败 [id=%d]: %v", e.ID, err)
			username = "[解密失败]"
		}

		password, err := a.decrypt(e.Password)
		if err != nil {
			log.Error("解密密码失败 [id=%d]: %v", e.ID, err)
			password = "[解密失败]"
		}

		note, err := a.decrypt(e.Note)
		if err != nil {
			log.Error("解密备注失败 [id=%d]: %v", e.ID, err)
			note = "[解密失败]"
		}

		result = append(result, EntryVO{
			ID:         e.ID,
			Title:      title,
			URL:        url,
			Username:   username,
			Password:   password,
			Note:       note,
			IsFavorite: e.IsFavorite,
		})
	}
	log.Debug("查询密码条目数: %d (filter=%s, sortBy=%s)", len(result), filter, sortBy)
	return result, nil
}

// 删除密码
func (a *App) DeletePassword(id uint) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isUnlocked {
		return errors.New("未解锁")
	}
	var entry PasswordEntry
	if err := a.db.Where("id = ?", id).First(&entry).Error; err != nil {
		return errors.New("密码不存在")
	}
	if err := a.db.Delete(&entry).Error; err != nil {
		return err
	}
	log.Info("删除密码条目 [id=%d]", id)
	return nil
}

// 修改密码
func (a *App) UpdatePassword(id uint, title, url, username, password, note string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isUnlocked {
		return errors.New("未解锁")
	}
	var entry PasswordEntry
	if err := a.db.Where("id = ?", id).First(&entry).Error; err != nil {
		return errors.New("密码不存在")
	}
	encTitle, err := a.encrypt(title)
	if err != nil {
		return err
	}
	encURL, err := a.encrypt(url)
	if err != nil {
		return err
	}
	encUsername, err := a.encrypt(username)
	if err != nil {
		return err
	}
	encPassword, err := a.encrypt(password)
	if err != nil {
		return err
	}
	encNote, err := a.encrypt(note)
	if err != nil {
		return err
	}
	entry.Title = encTitle
	entry.URL = encURL
	entry.Username = encUsername
	entry.Password = encPassword
	entry.Note = encNote
	if err := a.db.Save(&entry).Error; err != nil {
		return err
	}
	log.Info("更新密码条目 [id=%d]", id)
	return nil
}

// 搜索指定密码
// 根据标题、用户名、URL进行搜索，超时10秒无结果则返回失败
func (a *App) SearchPassword(keyword string) ([]EntryVO, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isUnlocked {
		return nil, errors.New("未解锁")
	}
	if len(keyword) == 0 {
		return nil, errors.New("搜索关键字不能为空")
	}

	// 加密字段无法直接LIKE，只能先查全部，再解密过滤
	var entries []PasswordEntry
	errCh := make(chan error, 1)
	resultCh := make(chan []EntryVO, 1)

	go func() {
		err := a.db.Find(&entries).Error
		if err != nil {
			errCh <- err
			return
		}
		var result []EntryVO
		for _, e := range entries {
			title, err := a.decrypt(e.Title)
			if err != nil {
				log.Error("搜索时解密标题失败 [id=%d]: %v", e.ID, err)
				continue
			}
			username, err := a.decrypt(e.Username)
			if err != nil {
				log.Error("搜索时解密用户名失败 [id=%d]: %v", e.ID, err)
				continue
			}
			url, err := a.decrypt(e.URL)
			if err != nil {
				log.Error("搜索时解密URL失败 [id=%d]: %v", e.ID, err)
				url = "[解密失败]"
			}

			// 在标题、用户名、URL中搜索
			if containsIgnoreCase(title, keyword) ||
				containsIgnoreCase(username, keyword) ||
				containsIgnoreCase(url, keyword) {
				password, err := a.decrypt(e.Password)
				if err != nil {
					log.Error("搜索时解密密码失败 [id=%d]: %v", e.ID, err)
					password = "[解密失败]"
				}
				note, err := a.decrypt(e.Note)
				if err != nil {
					log.Error("搜索时解密备注失败 [id=%d]: %v", e.ID, err)
					note = "[解密失败]"
				}
				result = append(result, EntryVO{
					ID:         e.ID,
					Title:      title,
					URL:        url,
					Username:   username,
					Password:   password,
					Note:       note,
					IsFavorite: e.IsFavorite,
				})
			}
		}
		resultCh <- result
	}()

	select {
	case err := <-errCh:
		return nil, err
	case result := <-resultCh:
		log.Debug("搜索关键词: %s, 结果数: %d", keyword, len(result))
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
	log.Info("主密码初始化成功")
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
	log.Info("应用已解锁")
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
	log.Info("应用已锁定")
}

// GetPasswordStrength 获取密码强度（供前端调用）
func (a *App) GetPasswordStrength(password string) utils.PasswordStrength {
	return utils.CheckPasswordStrength(password)
}

// GetPasswordCounts 获取各类密码数量
func (a *App) GetPasswordCounts() (map[string]int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isUnlocked {
		return nil, errors.New("未解锁")
	}
	counts := make(map[string]int64)
	var c int64

	// 全部
	a.db.Model(&PasswordEntry{}).Count(&c)
	counts["all"] = c
	// 收藏
	a.db.Model(&PasswordEntry{}).Where("is_favorite = ?", true).Count(&c)
	counts["favorite"] = c
	// 最近添加（取最新的10条作为最近）
	a.db.Model(&PasswordEntry{}).Where("id > (SELECT MAX(id) - 10 FROM password_entries)").Count(&c)
	counts["recent"] = c
	log.Debug("查询密码统计: %+v", counts)
	return counts, nil
}

// ToggleFavorite 切换收藏状态
func (a *App) ToggleFavorite(id uint) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isUnlocked {
		return errors.New("未解锁")
	}
	var entry PasswordEntry
	if err := a.db.Where("id = ?", id).First(&entry).Error; err != nil {
		return errors.New("密码条目不存在")
	}
	entry.IsFavorite = !entry.IsFavorite
	if err := a.db.Save(&entry).Error; err != nil {
		return err
	}
	log.Info("切换收藏状态 [id=%d, favorite=%v]", id, entry.IsFavorite)
	return nil
}
