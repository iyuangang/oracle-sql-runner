package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 创建测试配置文件
	testConfig := `{
        "databases": {
            "test": {
                "name": "Test DB",
                "user": "test_user",
                "password": "test_password",
                "host": "localhost",
                "port": 1521,
                "service": "TEST",
                "auto_commit": true,
                "max_retries": 3,
                "timeout_seconds": 30,
                "enable_dbms_output": true
            }
        },
        "execution": {
            "parallel_degree": 4,
            "batch_size": 1000,
            "max_file_size": 104857600,
            "retry_interval_seconds": 5
        }
    }`

	tmpfile, err := os.CreateTemp("", "config.*.json")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(testConfig)); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}
	tmpfile.Close()

	// 测试加载配置
	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证配置
	if len(cfg.Databases) != 1 {
		t.Errorf("期望1个数据库配置，实际得到%d个", len(cfg.Databases))
	}

	testDB, ok := cfg.Databases["test"]
	if !ok {
		t.Fatal("未找到测试数据库配置")
	}

	// 验证数据库配置
	if testDB.Name != "Test DB" {
		t.Errorf("数据库名称不匹配，期望 'Test DB'，实际得到 '%s'", testDB.Name)
	}

	// 验证执行配置
	if cfg.Execution.ParallelDegree != 4 {
		t.Errorf("并行度不匹配，期望 4，实际得到 %d", cfg.Execution.ParallelDegree)
	}
}

func TestPasswordEncryption(t *testing.T) {
	// 测试密码加密
	// ... 实现密码加密测试 ...
}
