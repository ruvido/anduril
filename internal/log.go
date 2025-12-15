package internal

import (
	"fmt"
	"os"
	"sync"
)

type Logger struct {
	mu sync.Mutex
	f  *os.File
}

func NewLogger(path string) (*Logger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &Logger{f: f}, nil
}

func (l *Logger) Log(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.f, format+"\n", args...)
}

func (l *Logger) Close() error {
	return l.f.Close()
}
