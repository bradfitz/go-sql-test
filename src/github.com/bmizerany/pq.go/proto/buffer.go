package proto

import (
	"bytes"
)

type Buffer struct {
	*bytes.Buffer
}

func NewBuffer(b []byte) *Buffer {
	return &Buffer{bytes.NewBuffer(b)}
}

func (b *Buffer) WriteCString(s string) {
	b.WriteString(s + "\x00")
}

func (b *Buffer) WriteInt16(v int16) {
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

func (b *Buffer) WriteInt32(v int32) {
	b.WriteByte(byte(v >> 24))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

// Reading

func (b *Buffer) ReadCString() string {
	s, err := b.ReadString(0)
	if err != nil {
		panic(err) // TODO: probably shouldn't panic
	}

	l := len(s)
	if l > 0 {
		s = s[:l-1]
	}

	return s
}

func (b *Buffer) ReadInt16() (i int16) {
	tmp := make([]byte, 2)
	_, err := b.Read(tmp)
	if err != nil {
		panic(err) // TODO: probably shouldn't panic
	}

	i = int16(tmp[0]) << 8
	i |= int16(tmp[1])
	return
}

func (b *Buffer) ReadInt32() (i int32) {
	tmp := make([]byte, 4)
	_, err := b.Read(tmp)
	if err != nil {
		panic(err) // TODO: probably shouldn't panic
	}

	i = int32(tmp[0]) << 24
	i |= int32(tmp[1]) << 16
	i |= int32(tmp[2]) << 8
	i |= int32(tmp[3])

	return
}

func (b *Buffer) ReadByte() byte {
	c, err := b.Buffer.ReadByte()
	if err != nil {
		panic(err)
	}
	return c
}
