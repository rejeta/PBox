package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"PasswordBox/internal/config"
)

// 新版数据结构
type PasswordEntry struct {
	ID         uint   `gorm:"primaryKey"`
	Title      string // 加密
	URL        string // 加密
	Username   string // 加密
	Password   string // 加密
	Note       string // 加密
	IsFavorite bool
}

// 旧版数据结构
type OldPasswordEntry struct {
	ID       uint
	UserID   uint
	Site     string // 加密，对应新版 Title
	Account  string // 加密，对应新版 Username
	Password string // 加密
}

func (OldPasswordEntry) TableName() string {
	return "password_entries_old"
}

var aesKey []byte

func encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(aesKey)
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

func decrypt(cryptoText string) (string, error) {
	block, err := aes.NewCipher(aesKey)
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

func main() {
	var (
		oldDb   = flag.String("old-db", "Box.db", "旧版数据库文件名")
		workDir = flag.String("work-dir", ".", "工作目录（存放盐值文件和数据库）")
	)
	flag.Parse()

	dbPath := filepath.Join(*workDir, *oldDb)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "数据库文件不存在: %s\n", dbPath)
		os.Exit(1)
	}

	// 获取主密码
	password := os.Getenv("PASSWORDBOX_MASTER_PASSWORD")
	if password == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("请输入主密码: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取密码失败: %v\n", err)
			os.Exit(1)
		}
		password = strings.TrimSpace(input)
	}

	if password == "" {
		fmt.Fprintln(os.Stderr, "密码不能为空")
		os.Exit(1)
	}

	// 解锁密钥
	keyConfig, err := config.Unlock(password, *workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解锁失败: %v\n", err)
		os.Exit(1)
	}
	aesKey = keyConfig.DerivedKey

	// 备份原数据库
	backupName := fmt.Sprintf("%s.bak.%s", *oldDb, time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(*workDir, backupName)
	if err := copyFile(dbPath, backupPath); err != nil {
		fmt.Fprintf(os.Stderr, "备份数据库失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已备份原数据库到: %s\n", backupPath)

	// 打开数据库
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开数据库失败: %v\n", err)
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取底层连接失败: %v\n", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	// 检查旧表是否存在
	var oldTableCount int64
	db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='password_entries'").Scan(&oldTableCount)
	if oldTableCount == 0 {
		fmt.Fprintln(os.Stderr, "未找到旧版 password_entries 表，可能已完成迁移或数据库为空")
		os.Exit(1)
	}

	// 检查是否为旧版表结构（包含 user_id 列）
	var userIDCount int64
	db.Raw("SELECT count(*) FROM pragma_table_info('password_entries') WHERE name='user_id'").Scan(&userIDCount)
	if userIDCount == 0 {
		fmt.Fprintln(os.Stderr, "当前 password_entries 表不含 user_id 列，可能已经迁移到新版本")
		os.Exit(1)
	}

	// 将旧表重命名
	if err := db.Exec("ALTER TABLE password_entries RENAME TO password_entries_old").Error; err != nil {
		fmt.Fprintf(os.Stderr, "重命名旧表失败: %v\n", err)
		os.Exit(1)
	}

	// 创建新表
	if err := db.AutoMigrate(&PasswordEntry{}); err != nil {
		fmt.Fprintf(os.Stderr, "创建新表失败: %v\n", err)
		os.Exit(1)
	}

	// 读取旧记录
	var oldEntries []OldPasswordEntry
	if err := db.Find(&oldEntries).Error; err != nil {
		fmt.Fprintf(os.Stderr, "读取旧记录失败: %v\n", err)
		os.Exit(1)
	}

	if len(oldEntries) == 0 {
		fmt.Println("旧表中没有记录，无需迁移数据")
	} else {
		// 验证主密码：解密第一条记录的 Site 字段，检查是否为有效 UTF-8
		firstSite, err := decrypt(oldEntries[0].Site)
		if err != nil || !utf8.ValidString(firstSite) {
			fmt.Fprintln(os.Stderr, "主密码可能不正确，无法正确解密旧数据。迁移已中止。")
			os.Exit(1)
		}

		// 迁移数据
		for i, old := range oldEntries {
			sitePlain, _ := decrypt(old.Site)
			accountPlain, _ := decrypt(old.Account)
			passwordPlain, _ := decrypt(old.Password)

			newTitle, _ := encrypt(sitePlain)
			newUsername, _ := encrypt(accountPlain)
			newPassword, _ := encrypt(passwordPlain)
			newURL, _ := encrypt("")
			newNote, _ := encrypt("")

			entry := PasswordEntry{
				Title:      newTitle,
				URL:        newURL,
				Username:   newUsername,
				Password:   newPassword,
				Note:       newNote,
				IsFavorite: false,
			}
			if err := db.Create(&entry).Error; err != nil {
				fmt.Fprintf(os.Stderr, "插入新记录失败 [index=%d]: %v\n", i, err)
				os.Exit(1)
			}
		}
		fmt.Printf("成功迁移 %d 条记录\n", len(oldEntries))
	}

	// 删除旧表和 users 表
	db.Exec("DROP TABLE IF EXISTS password_entries_old")
	db.Exec("DROP TABLE IF EXISTS users")

	fmt.Println("迁移完成！")
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}
