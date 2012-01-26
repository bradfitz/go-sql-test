package proto

import (
	"bytes"
	"github.com/bmizerany/assert"
	"io"
	"testing"
)

func TestScanSimple(t *testing.T) {
	b := bytes.NewBufferString("X\x00\x00\x00\x0Ctesting\x00")
	s := scan(b, nil)
	got := <-s.msgs
	assert.Equal(t, Header{'X', 8}, got.Header)
	assert.Equal(t, "testing\x00", got.String())

	_, ok := <-s.msgs
	assert.Equal(t, false, ok)
	assert.Equal(t, io.EOF, s.err)
}
