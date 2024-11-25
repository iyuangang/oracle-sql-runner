package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/godror/godror"
)

type Config struct {
    User     string `json:"user"`
    Password string `json:"password"`
    Host     string `json:"host"`
    Port     int    `json:"port"`
    Service  string `json:"service"`
}

type Result struct {
    Success int
    Failed  int
    Errors  []string
}

func executeSQLFile(db *sql.DB, filepath string) Result {
    result := Result{}
    file, err := os.Open(filepath)
    if err != nil {
        log.Fatalf("打开SQL文件失败: %v", err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    var sqlBuffer strings.Builder
    inPLSQLBlock := false

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())

        // 跳过空行和注释
        if line == "" || strings.HasPrefix(line, "--") {
            continue
        }

        upperLine := strings.ToUpper(line)
        
        // 检查是否进入PL/SQL块
        if strings.HasPrefix(upperLine, "BEGIN") ||
           strings.HasPrefix(upperLine, "DECLARE") {
            inPLSQLBlock = true
        }

        // 特殊处理CREATE语句
        if strings.HasPrefix(upperLine, "CREATE OR REPLACE") {
            if strings.Contains(upperLine, "PROCEDURE") ||
               strings.Contains(upperLine, "FUNCTION") ||
               strings.Contains(upperLine, "TRIGGER") ||
               strings.Contains(upperLine, "PACKAGE") {
                inPLSQLBlock = true
            }
        }

        sqlBuffer.WriteString(line)
        sqlBuffer.WriteString("\n")

        // 检查PL/SQL块结束
        if inPLSQLBlock && line == "/" {
            sql := strings.TrimSpace(sqlBuffer.String())
            sql = strings.TrimSuffix(sql, "/")
            
            fmt.Printf("\n执行PL/SQL块:\n%s\n", sql)
            
            if _, err := db.Exec(sql); err != nil {
                result.Failed++
                result.Errors = append(result.Errors, fmt.Sprintf("PL/SQL执行失败: %v\nSQL: %s", err, sql))
            } else {
                result.Success++
            }

            sqlBuffer.Reset()
            inPLSQLBlock = false
            continue
        }

        // 处理普通SQL语句
        if !inPLSQLBlock && strings.HasSuffix(line, ";") {
            sql := strings.TrimSpace(sqlBuffer.String())
            sql = strings.TrimSuffix(sql, ";")

            if strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
                if err := executeQuery(db, sql); err != nil {
                    result.Failed++
                    result.Errors = append(result.Errors, fmt.Sprintf("查询执行失败: %v\nSQL: %s", err, sql))
                } else {
                    result.Success++
                }
            } else {
                fmt.Printf("\n执行SQL:\n%s\n", sql)
                if _, err := db.Exec(sql); err != nil {
                    result.Failed++
                    result.Errors = append(result.Errors, fmt.Sprintf("SQL执行失败: %v\nSQL: %s", err, sql))
                } else {
                    result.Success++
                }
            }

            sqlBuffer.Reset()
        }
    }

    // 处理最后一条SQL（如果有）
    if sqlBuffer.Len() > 0 {
        sql := strings.TrimSpace(sqlBuffer.String())
        if !inPLSQLBlock {  // 只处理非PL/SQL块的最后一条语句
            if strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
                if err := executeQuery(db, sql); err != nil {
                    result.Failed++
                    result.Errors = append(result.Errors, fmt.Sprintf("查询执行失败: %v\nSQL: %s", err, sql))
                } else {
                    result.Success++
                }
            } else {
                if _, err := db.Exec(sql); err != nil {
                    result.Failed++
                    result.Errors = append(result.Errors, fmt.Sprintf("SQL执行失败: %v\nSQL: %s", err, sql))
                } else {
                    result.Success++
                }
            }
        }
    }

    return result
}

func executeQuery(db *sql.DB, query string) error {
    fmt.Printf("\n执行查询:\n%s\n", query)
    
    rows, err := db.Query(query)
    if err != nil {
        return err
    }
    defer rows.Close()

    columns, err := rows.Columns()
    if err != nil {
        return err
    }

    // 打印列头
    for i, col := range columns {
        if i > 0 {
            fmt.Print("\t")
        }
        fmt.Print(col)
    }
    fmt.Println()
    fmt.Println(strings.Repeat("-", 80))

    values := make([]interface{}, len(columns))
    scanArgs := make([]interface{}, len(columns))
    for i := range values {
        scanArgs[i] = &values[i]
    }

    rowCount := 0
    for rows.Next() {
        err := rows.Scan(scanArgs...)
        if err != nil {
            return err
        }

        for i, value := range values {
            if i > 0 {
                fmt.Print("\t")
            }
            switch v := value.(type) {
            case nil:
                fmt.Print("NULL")
            case []byte:
                fmt.Print(string(v))
            default:
                fmt.Print(v)
            }
        }
        fmt.Println()
        rowCount++
    }

    fmt.Printf("\n共返回 %d 行数据\n", rowCount)
    fmt.Println(strings.Repeat("-", 80))

    return rows.Err()
}

func main() {
    configFile := flag.String("c", "config.json", "配置文件路径")
    sqlFile := flag.String("f", "", "SQL文件路径")
    flag.Parse()

    if *sqlFile == "" {
        log.Fatal("请指定SQL文件路径 (-f)")
    }

    data, err := os.ReadFile(*configFile)
    if err != nil {
        log.Fatalf("读取配置文件失败: %v", err)
    }

    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        log.Fatalf("解析配置文件失败: %v", err)
    }

    connStr := fmt.Sprintf(`user="%s" password="%s" connectString="%s:%d/%s"`,
        cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Service)

    db, err := sql.Open("godror", connStr)
    if err != nil {
        log.Fatalf("连接数据库失败: %v", err)
    }
    defer db.Close()

    if err := db.Ping(); err != nil {
        log.Fatalf("验证数据库连接失败: %v", err)
    }

    result := executeSQLFile(db, *sqlFile)

    fmt.Printf("\n执行结果:\n")
    fmt.Printf("成功: %d\n", result.Success)
    fmt.Printf("失败: %d\n", result.Failed)
    if result.Failed > 0 {
        fmt.Printf("\n错误详情:\n")
        for i, err := range result.Errors {
            fmt.Printf("%d. %s\n", i+1, err)
        }
    }
}
