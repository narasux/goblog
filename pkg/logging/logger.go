package logging

import (
	"io"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/narasux/goblog/pkg/envs"
)

// 获取日志 Writer，这里返回双写 Writer（stdout & file）
func getWriter(logType string) (io.Writer, error) {
	// 标准输出
	stdoutWriter, _ := getOSWriter()
	// 文件日志
	fileWriter, err := getFileWriter(logType)
	if err != nil {
		return nil, err
	}
	return io.MultiWriter(stdoutWriter, fileWriter), nil
}

func getOSWriter() (io.Writer, error) {
	return os.Stdout, nil
}

func getFileWriter(logType string) (io.Writer, error) {
	// 不同的日志类型分目录存储
	path := filepath.Join(envs.LogFileBaseDir, logType)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.MkdirAll(path, os.ModePerm); err != nil {
			return nil, err
		}
	}
	filename := logType + ".log"

	// 使用 lumberjack 实现日志切割归档，参数都先写死，暂时没有放出来的意义
	writer := &lumberjack.Logger{
		Filename: filepath.Join(path, filename),
		// megabytes
		MaxSize:    128,
		MaxBackups: 10,
		// days
		MaxAge:    14,
		LocalTime: true,
	}
	return writer, nil
}
