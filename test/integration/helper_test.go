package integration

import (
	"database/sql"
	"os"
	"testing"
)

func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()

    // 从环境变量获取连接信息
    dsn := os.Getenv("TEST_DB_DSN")
    if dsn == "" {
        dsn = "test/test123@//localhost:1521/XE"
    }

    db, err := sql.Open("godror", dsn)
    if err != nil {
        t.Fatalf("连接数据库失败: %v", err)
    }

    return db
}

func cleanupTestDB(t *testing.T, db *sql.DB) {
    t.Helper()
    
    // 清理测试数据
    _, err := db.Exec(`
        BEGIN
            FOR t IN (SELECT table_name FROM user_tables) LOOP
                EXECUTE IMMEDIATE 'DROP TABLE ' || t.table_name || ' CASCADE CONSTRAINTS';
            END LOOP;
        END;
    `)
    if err != nil {
        t.Errorf("清理数据库失败: %v", err)
    }
    
    db.Close()
} 
