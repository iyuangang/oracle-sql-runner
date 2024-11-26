package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/iyuangang/oracle-sql-runner/pkg/models"
)

func TestParseFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sql-parser-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		content  string
		wantErr  bool
		expected []models.SQLTask
	}{
		{
			name: "基本SQL语句",
			content: `  -- 注释
    SELECT * FROM users;
    INSERT INTO users (name) VALUES ('test');`,
			wantErr: false,
			expected: []models.SQLTask{
				{
					SQL:      "SELECT * FROM users",
					Type:     models.SQLTypeQuery,
					LineNum:  2,
					Filename: "", // 将在测试中设置
				},
				{
					SQL:      "INSERT INTO users (name) VALUES ('test')",
					Type:     models.SQLTypeExec,
					LineNum:  3,
					Filename: "", // 将在测试中设置
				},
			},
		},
		{
			name: "PL/SQL块",
			content: `CREATE OR REPLACE PROCEDURE test_proc AS
BEGIN
    DBMS_OUTPUT.PUT_LINE('Hello');
END;
/

CREATE OR REPLACE FUNCTION test_func RETURN NUMBER AS
BEGIN
    RETURN 1;
END;
/`,
			wantErr: false,
			expected: []models.SQLTask{
				{
					SQL: `CREATE OR REPLACE PROCEDURE test_proc AS
BEGIN
    DBMS_OUTPUT.PUT_LINE('Hello');
END;`,
					Type:     models.SQLTypePLSQL,
					LineNum:  5,
					Filename: "", // 将在测试中设置
				},
				{
					SQL: `CREATE OR REPLACE FUNCTION test_func RETURN NUMBER AS
BEGIN
    RETURN 1;
END;`,
					Type:     models.SQLTypePLSQL,
					LineNum:  11,
					Filename: "", // 将在测试中设置
				},
			},
		},
		{
			name: "混合SQL和PL/SQL",
			content: `-- 创建表
CREATE TABLE test_table (id NUMBER);

-- 创建触发器
CREATE OR REPLACE TRIGGER test_trigger
    BEFORE INSERT ON test_table
BEGIN
    NULL;
END;
/

-- 插入数据
INSERT INTO test_table VALUES (1);`,
			wantErr: false,
			expected: []models.SQLTask{
				{
					SQL:      "CREATE TABLE test_table (id NUMBER)",
					Type:     models.SQLTypeExec,
					LineNum:  2,
					Filename: "", // 将在测试中设置
				},
				{
					SQL: `CREATE OR REPLACE TRIGGER test_trigger
    BEFORE INSERT ON test_table
BEGIN
    NULL;
END;`,
					Type:     models.SQLTypePLSQL,
					LineNum:  10,
					Filename: "", // 将在测试中设置
				},
				{
					SQL:      "INSERT INTO test_table VALUES (1)",
					Type:     models.SQLTypeExec,
					LineNum:  13,
					Filename: "", // 将在测试中设置
				},
			},
		},
		{
			name: "包声明和主体",
			content: `CREATE OR REPLACE PACKAGE test_pkg AS
    PROCEDURE test_proc;
END;
/

CREATE OR REPLACE PACKAGE BODY test_pkg AS
    PROCEDURE test_proc IS
BEGIN
    NULL;
END;
END;
/`,
			wantErr: false,
			expected: []models.SQLTask{
				{
					SQL: `CREATE OR REPLACE PACKAGE test_pkg AS
    PROCEDURE test_proc;
END;`,
					Type:     models.SQLTypePLSQL,
					LineNum:  4,
					Filename: "", // 将在测试中设置
				},
				{
					SQL: `CREATE OR REPLACE PACKAGE BODY test_pkg AS
    PROCEDURE test_proc IS
BEGIN
    NULL;
END;
END;`,
					Type:     models.SQLTypePLSQL,
					LineNum:  12,
					Filename: "", // 将在测试中设置
				},
			},
		},
		{
			name: "空文件",
			content: `-- 只有注释
-- 和空行

`,
			wantErr:  false,
			expected: make([]models.SQLTask, 0),
		},
		{
			name: "DECLARE块",
			content: `DECLARE
    v_count NUMBER;
BEGIN
    IF 0 = 1 THEN
        DBMS_OUTPUT.PUT_LINE('This should not be printed');
    ELSE
        SELECT COUNT(*) INTO v_count FROM dual;
    END IF;
END;
/`,
			wantErr: false,
			expected: []models.SQLTask{
				{
					SQL: `DECLARE
    v_count NUMBER;
BEGIN
    IF 0 = 1 THEN
    DBMS_OUTPUT.PUT_LINE('This should not be printed');
    ELSE
    SELECT COUNT(*) INTO v_count FROM dual;
END IF;
END;`,
					Type:     models.SQLTypePLSQL,
					LineNum:  10,
					Filename: "", // 将在测试中设置
				},
			},
		},
		{
			name:    "单条语句无分号",
			content: `SELECT * FROM dual`,
			wantErr: false,
			expected: []models.SQLTask{
				{
					SQL:      "SELECT * FROM dual",
					Type:     models.SQLTypeQuery,
					LineNum:  1,
					Filename: "", // 将在测试中设置
				},
			},
		},
		{
			name:    "单条DML语句无分号",
			content: `INSERT INTO test_table VALUES (1)`,
			wantErr: false,
			expected: []models.SQLTask{
				{
					SQL:      "INSERT INTO test_table VALUES (1)",
					Type:     models.SQLTypeExec,
					LineNum:  1,
					Filename: "", // 将在测试中设置
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试文件
			tmpFile := filepath.Join(tmpDir, "test.sql")
			err := os.WriteFile(tmpFile, []byte(tt.content), 0o644)
			if err != nil {
				t.Fatalf("创建测试文件失败: %v", err)
			}

			// 更新预期结果中的文件名
			for i := range tt.expected {
				tt.expected[i].Filename = tmpFile
			}

			// 执行测试
			got, err := ParseFile(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 比较结果
			if !reflect.DeepEqual(got, tt.expected) {
				// 特殊处理空切片的情况
				if len(got) == 0 && len(tt.expected) == 0 {
					return // 两个都是空切片，视为相等
				}

				t.Errorf("ParseFile() got = %#v, want %#v", got, tt.expected)
				for i := range got {
					if i < len(tt.expected) {
						t.Errorf("\n实际结果 [%d]: SQL=%q, Type=%v, LineNum=%d\n预期结果 [%d]: SQL=%q, Type=%v, LineNum=%d",
							i, got[i].SQL, got[i].Type, got[i].LineNum,
							i, tt.expected[i].SQL, tt.expected[i].Type, tt.expected[i].LineNum)
					}
				}
			}
		})
	}

	// 测试文件不存在的情况
	t.Run("文件不存在", func(t *testing.T) {
		_, err := ParseFile("non_existent_file.sql")
		if err == nil {
			t.Error("ParseFile() 应该返回错误，但没有")
		}
	})
}
