package proto

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
)

type Type byte

const (
	Statement = Type('S')
	Portal    = Type('S')
)

const ProtoVersion = int32(196608)

type Values map[string]string

func (vs Values) Get(k string) string {
	if v, ok := vs[k]; ok {
		return v
	}
	return ""
}

func (vs Values) Set(k, v string) {
	vs[k] = v
}

func (vs Values) Del(k string) {
	delete(vs, k)
}

type Conn struct {
	b   *Buffer
	scr *scanner
	wc  io.ReadWriteCloser
}

func New(rwc io.ReadWriteCloser, notifies chan<- *Notify) *Conn {
	cn := &Conn{
		b:   NewBuffer(nil),
		wc:  rwc,
		scr: scan(rwc, notifies),
	}

	return cn
}

func (cn *Conn) Close() error {
	return cn.wc.Close()
}

func (cn *Conn) Next() (*Msg, error) {
	m, ok := <-cn.scr.msgs
	if !ok {
		return nil, cn.scr.err
	}
	if err := m.parse(); err != nil {
		return nil, err
	}
	return m, nil
}

func (cn *Conn) Startup(params Values) error {
	cn.b.WriteInt32(ProtoVersion)
	for k, v := range params {
		cn.b.WriteCString(k)
		cn.b.WriteCString(v)
	}
	cn.b.WriteCString("")
	return cn.flush(0)
}

func (cn *Conn) Password(pw string) error {
	cn.b.WriteCString(pw)
	return cn.flush('p')
}

func md5s(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func concat(a, b string) string {
	return a + b
}

func (cn *Conn) PasswordMd5(salt, user, pw string) error {
	// in SQL: concat('md5', md5(concat(md5(concat(password, username)), random-salt)))
	sum := concat("md5", md5s(concat(md5s(concat(pw, user)), salt)))
	cn.b.WriteCString(sum)
	return cn.flush('p')
}

func (cn *Conn) SimpleQuery(query string) error {
	cn.b.WriteCString(query)
	return cn.flush('Q')
}

func (cn *Conn) Parse(name, query string) error {
	cn.b.WriteCString(name)
	cn.b.WriteCString(query)
	cn.b.WriteInt16(0)
	return cn.flush('P')
}

func (cn *Conn) Bind(portal, stmt string, args ...string) error {
	cn.b.WriteCString(portal)
	cn.b.WriteCString(stmt)

	// TODO: Use format codes; maybe?
	//       some thought needs to be put into the design of this.
	//       See (Bind) http://developer.postgresql.org/pgdocs/postgres/protocol-message-formats.html
	cn.b.WriteInt16(0)

	cn.b.WriteInt16(int16(len(args)))
	for _, arg := range args {
		cn.b.WriteInt32(int32(len(arg)))
		cn.b.WriteString(arg)
	}

	// TODO: Use result format codes; maybe?
	//       some thought needs to be put into the design of this.
	//       See (Bind) http://developer.postgresql.org/pgdocs/postgres/protocol-message-formats.html
	cn.b.WriteInt16(0)

	return cn.flush('B')
}

func (cn *Conn) Execute(name string, rows int) error {
	cn.b.WriteCString(name)
	cn.b.WriteInt32(int32(rows))
	return cn.flush('E')
}

func (cn *Conn) Describe(t Type, name string) error {
	cn.b.WriteByte(byte(t))
	cn.b.WriteCString(name)
	return cn.flush('D')
}

func (cn *Conn) Sync() error {
	err := cn.flush('S')
	if err != nil {
		return err
	}
	return nil
}

func (cn *Conn) ClosePP(t Type, name string) error {
	cn.b.WriteByte(byte(t))
	cn.b.WriteCString(name)
	return cn.flush('C')
}

func (cn *Conn) flush(t byte) error {
	if t > 0 {
		err := binary.Write(cn.wc, binary.BigEndian, t)
		if err != nil {
			return err
		}
	}

	l := int32(cn.b.Len()) + sizeOfInt32
	err := binary.Write(cn.wc, binary.BigEndian, l)
	if err != nil {
		return err
	}

	_, err = cn.b.WriteTo(cn.wc)
	if err != nil {
		return err
	}

	return err
}
