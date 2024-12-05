package utils

import (
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
			password: "/X0c76bYY8S0i5hQ7XbciA8/CSgI",
			wantErr:  false,
			checkFunc: func(decrypted string) bool {
				return decrypted == "hello"
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
			name:     "有效的base64但不是加密数据",
			password: "MTIzNDU2Nzg5MDEyMzQ1Ng==", // 16字节的随机数的base64
			want:     false,
		},
		{
			name:     "超长的base64字符串",
			password: strings.Repeat("QQ==", 1000), // 过长的base64字符串
			want:     false,
		},
		{
			name:     "只有IV长度的base64",
			password: "YWJjZGVmZ2hpamtsbW5vcA==", // 16字节的base64
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
