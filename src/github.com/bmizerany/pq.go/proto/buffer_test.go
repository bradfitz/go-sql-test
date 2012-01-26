package proto

import (
	"bytes"
	"testing"
	"github.com/bmizerany/assert"
)

func TestBufferSimple(t *testing.T) {
	buf := Buffer{new(bytes.Buffer)}
	buf.WriteCString("testing")
	buf.WriteInt16(1)
	buf.WriteInt32(2)
	assert.Equal(t, "testing\x00\x00\x01\x00\x00\x00\x02", buf.String())

	buf = Buffer{bytes.NewBuffer(buf.Bytes())}
	assert.Equal(t, "testing", buf.ReadCString())
	assert.Equal(t, int16(1), buf.ReadInt16())
	assert.Equal(t, int32(2), buf.ReadInt32())
}
