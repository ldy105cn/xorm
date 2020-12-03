package xorm

/*
use for err_logger
*/

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var dir = "./err_sqls"

type tableWriter struct {
	fileName string
	file     *os.File
}

type errWriter struct {
	handles map[string]*tableWriter
}

// Init init directory
func (w *errWriter) Init() {
	if err := os.MkdirAll(dir, 0774); err != nil {
		fmt.Printf("创建sql错误文件失败:%s", err)
	}
}

func getFileName(tableName string) string {
	now := time.Now()
	today := now.Format("2006-01-02")
	return fmt.Sprintf("%s/%s_%s.sql", dir, tableName, today)
}

func (w *errWriter) createTableWriter(tableName string) *tableWriter {
	fileName := getFileName(tableName)
	var err error
	writer := tableWriter{
		fileName: fileName,
	}
	entry := logrus.WithField("fileName", fileName)
	writer.file, err = os.OpenFile(fileName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)

	if err != nil {
		entry.WithError(err).Errorln("打开文件失败")
		return nil
	}

	return &writer
}

func (w *errWriter) getTableWriter(tableName string) *tableWriter {
	entry := logrus.WithFields(logrus.Fields{"tableName": tableName})
	writer, ok := w.handles[tableName]
	if !ok {
		writer = w.createTableWriter(tableName)
	}
	if writer == nil {
		entry.Errorln("无法获取tableWriter")
		return nil
	}
	fileName := getFileName(tableName)
	if fileName != writer.fileName {
		writer.file.Close()
		writer = w.createTableWriter(tableName)
	}
	return writer
}

func (w *errWriter) Write(tableName, sql string) {
	entry := logrus.WithFields(logrus.Fields{"tableName": tableName, "sql": sql})
	writer := w.getTableWriter(tableName)
	if writer == nil {
		entry.Errorln("无法获取tableWriter")
		return
	}
	cache := "-- " + time.Now().Format("2006-01-02 15:04:05") + "\n"
	cache += sql + ";\n"
	writer.file.Write([]byte(cache))
}
