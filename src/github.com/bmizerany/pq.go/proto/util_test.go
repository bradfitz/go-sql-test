package proto

import (
	"fmt"
	"io"
)

type lrwc struct {
	rwc io.ReadWriteCloser
}

func (l *lrwc) Write(b []byte) (int, error) {
	fmt.Printf(">> %q\n", b)
	return l.rwc.Write(b)
}

func (l *lrwc) Read(b []byte) (int, error) {
	fmt.Printf("<< %q\n", b)
	return l.rwc.Read(b)
}

func (l *lrwc) Close() error {
	fmt.Println("<closed>")
	return l.rwc.Close()
}
