package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// CreateDirIfNotExist 创建目录（如果不存在）
func CreateDirIfNotExist(path string) error {
	if !FileExists(path) {
		return os.MkdirAll(path, os.ModePerm)
	}
	return nil
}

// GetFileSize 获取文件大小
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// FormatDuration 格式化持续时间
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

// SplitSQLFile 分割大SQL文件
func SplitSQLFile(filePath string, maxSize int64) ([]string, error) {
	if maxSize <= 0 {
		return []string{filePath}, nil
	}

	fileSize, err := GetFileSize(filePath)
	if err != nil {
		return nil, err
	}

	if fileSize <= maxSize {
		return []string{filePath}, nil
	}

	// 创建临时目录
	tmpDir := filepath.Join(os.TempDir(), "sql-runner-split")
	if err := CreateDirIfNotExist(tmpDir); err != nil {
		return nil, err
	}

	// 读取源文件
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// 按语句分割
	statements := strings.Split(string(content), ";")
	var files []string
	var currentFile strings.Builder
	fileCount := 0

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// 如果当前文件超过大小限制，创建新文件
		if int64(currentFile.Len()) > maxSize {
			// 写入文件
			fileName := filepath.Join(tmpDir, fmt.Sprintf("part_%d.sql", fileCount))
			if err := os.WriteFile(fileName, []byte(currentFile.String()), 0o644); err != nil {
				return nil, err
			}
			files = append(files, fileName)
			fileCount++
			currentFile.Reset()
		}

		currentFile.WriteString(stmt + ";\n")
	}

	// 写入最后一个文件
	if currentFile.Len() > 0 {
		fileName := filepath.Join(tmpDir, fmt.Sprintf("part_%d.sql", fileCount))
		if err := os.WriteFile(fileName, []byte(currentFile.String()), 0o644); err != nil {
			return nil, err
		}
		files = append(files, fileName)
	}

	return files, nil
}
