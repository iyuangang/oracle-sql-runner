package utils

import (
	"crypto/aes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestEncryptPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "正常密码加密",
			password: "test123",
			wantErr:  false,
		},
		{
			name:     "空密码加密",
			password: "",
			wantErr:  true,
		},
		{
			name:     "特殊字符密码加密",
			password: "test!@#$%^&*()_+",
			wantErr:  false,
		},
		{
			name:     "中文密码加密",
			password: "测试密码123",
			wantErr:  false,
		},
		{
			name:     "超长密码加密",
			password: strings.Repeat("a", 1000),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncryptPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncryptPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == "" {
					t.Error("EncryptPassword() returned empty string")
				}
				if got == tt.password {
					t.Error("EncryptPassword() returned original password")
				}
				if !IsEncrypted(got) {
					t.Error("EncryptPassword() result is not properly encrypted")
				}
			}
		})
	}

	// 测试无效密钥
	t.Run("无效密钥", func(t *testing.T) {
		restore := setEncryptionKey([]byte{1}) // 设置一个无效的密钥
		defer restore()                        // 确保测试后恢复原始密钥

		_, err := EncryptPassword("test")
		if err == nil {
			t.Error("使用无效密钥时应该返回错误")
		}
	})
}

func TestDecryptPassword(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantErr   bool
		checkFunc func(string) bool
	}{
		{
			name:     "已知加密密码解密",
			password: "wyZpetSzmj0DQngd7pfkO1pw3PedA3rn",
			wantErr:  false,
			checkFunc: func(decrypted string) bool {
				return decrypted == "hello123"
			},
		},
		{
			name:     "空密文解密",
			password: "",
			wantErr:  true,
			checkFunc: func(decrypted string) bool {
				return true
			},
		},
		{
			name:     "无效的base64",
			password: "invalid base64!@#$",
			wantErr:  true,
			checkFunc: func(decrypted string) bool {
				return true
			},
		},
		{
			name:     "太短的密文",
			password: "aGVsbG8=", // "hello" 的 base64
			wantErr:  true,
			checkFunc: func(decrypted string) bool {
				return true
			},
		},
		{
			name:     "正常密码加解密",
			password: "test123",
			wantErr:  false,
			checkFunc: func(decrypted string) bool {
				return decrypted == "test123"
			},
		},
		{
			name:     "特殊字符加解密",
			password: "test!@#$%^&*()_+",
			wantErr:  false,
			checkFunc: func(decrypted string) bool {
				return decrypted == "test!@#$%^&*()_+"
			},
		},
		{
			name:     "中文密码加解密",
			password: "测试密码123",
			wantErr:  false,
			checkFunc: func(decrypted string) bool {
				return decrypted == "测试密码123"
			},
		},
		{
			name:     "无效的加密字符串",
			password: "invalid-encrypted-string",
			wantErr:  true,
			checkFunc: func(decrypted string) bool {
				return true
			},
		},
		{
			name:     "损坏的加密数据",
			password: "nM1vGnDe5YKx/h4MvPgRkrhiav2voyAYbrLHeQ=Y=",
			wantErr:  true,
			checkFunc: func(decrypted string) bool {
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "已知加密密码解密" {
				// 测试无效密钥
				restore := setEncryptionKey([]byte{1})
				_, err := DecryptPassword(tt.password)
				if err == nil {
					t.Error("使用无效密钥时应该返回错误")
				}
				restore()
			}

			var encrypted string
			var err error

			if tt.name == "已知加密密码解密" {
				// 对于已知的加密密码，直接使用
				encrypted = tt.password
			} else if !tt.wantErr {
				// 对于其他期望成功的测试，先加密
				encrypted, err = EncryptPassword(tt.password)
				if err != nil {
					t.Fatalf("加密失败: %v", err)
				}
			} else {
				// 对于期望失败的测试，直接使用测试数据
				encrypted = tt.password
			}

			got, err := DecryptPassword(encrypted)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecryptPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !tt.checkFunc(got) {
				t.Errorf("DecryptPassword() = %v, want %v", got, tt.password)
			}
		})
	}
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{
			name:     "已知加密密码",
			password: "/X0c76bYY8S0i5hQ7XbciA8/CSgI",
			want:     true,
		},
		{
			name:     "空密码",
			password: "",
			want:     false,
		},
		{
			name:     "未加密的密码",
			password: "test123",
			want:     false,
		},
		{
			name:     "无效的base64",
			password: "invalid-base64!@#",
			want:     false,
		},
		{
			name:     "有效的base64但长度不够",
			password: "dGVzdA==", // "test" 的 base64
			want:     false,
		},
		{
			name:     "只有IV的base64",
			password: base64.StdEncoding.EncodeToString(make([]byte, aes.BlockSize)),
			want:     false,
		},
		{
			name:     "IV加一个字节",
			password: base64.StdEncoding.EncodeToString(make([]byte, aes.BlockSize+1)),
			want:     true,
		},
		{
			name:     "超长密文",
			password: base64.StdEncoding.EncodeToString(make([]byte, 3000)),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEncrypted(tt.password); got != tt.want {
				t.Errorf("IsEncrypted() = %v, want %v for %s", got, tt.want, tt.name)
			}
		})
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"简单密码", "simple123", false},
		{"复杂密码", "Complex!@#$%^&*()_+", false},
		{"中文密码", "测试密码123", false},
		{"空密码", "", true},
		{"超长密码", strings.Repeat("a", 1000), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 加密
			encrypted, err := EncryptPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncryptPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// 验证加密结果
			if !IsEncrypted(encrypted) {
				t.Error("加密结果验证失败")
			}

			// 解密
			decrypted, err := DecryptPassword(encrypted)
			if err != nil {
				t.Fatalf("解密失败: %v", err)
			}

			// 验证解密结果
			if decrypted != tt.password {
				t.Errorf("解密结果不匹配: got %v, want %v", decrypted, tt.password)
			}
		})
	}
}

// TestCryptoErrors 测试加密/解密的错误情况
func TestCryptoErrors(t *testing.T) {
	// 保存原始的随机数读取器
	originalRandReader := rand.Reader
	defer func() {
		rand.Reader = originalRandReader
	}()

	// 测试随机数生成失败
	rand.Reader = &errorReader{}
	_, err := EncryptPassword("test")
	if err == nil {
		t.Error("随机数生成失败时应该返回错误")
	}
}

// errorReader 实现了 io.Reader 接口，用于测试错误情况
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("模拟的随机数生成错误")
}
