package xorm

import (
	"sync"
)

// ErrLoggerWriter error sql writer
type ErrLoggerWriter interface {
	Write(tableName, sql string)
}

// ErrLogger put sql to disk when update fail
type ErrLogger struct {
	enable bool
	output ErrLoggerWriter
	mu     sync.Mutex
}

// IsEnable is ErrLogger enable
func (l *ErrLogger) IsEnable() bool {
	return l.enable
}

// Write write sql to disk
func (l *ErrLogger) Write(tableName, sql string) {
	if !l.enable {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output.Write(tableName, sql)
}
