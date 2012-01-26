package proto

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
)

const sizeOfInt32 = int32(32 / 8)

type Notify struct {
	Pid     int
	From    string
	Payload string
}

type scanner struct {
	r        io.Reader
	msgs     <-chan *Msg
	notifies chan<- *Notify
	err      error
}

func scan(r io.Reader, notifies chan<- *Notify) *scanner {
	msgs := make(chan *Msg)
	s := &scanner{r: r, msgs: msgs, notifies: notifies}

	go s.run(msgs)

	return s
}

func (s *scanner) run(msgs chan<- *Msg) {
	var err error
	defer func() {
		s.err = err
		close(msgs)
	}()

	for {
		m := new(Msg)

		err = binary.Read(s.r, binary.BigEndian, &m.Header)
		if err != nil {
			return
		}
		m.Length -= sizeOfInt32

		b := make([]byte, m.Length)
		_, err = io.ReadFull(s.r, b)
		if err != nil {
			return
		}

		m.Buffer = &Buffer{bytes.NewBuffer(b)}

		switch m.Type {
		default:
			msgs <- m
		case 'N':
			m.parse()
			log.Printf("pq: NOTICE (%c) %s", m.Status, m.Message)
		case 'A': // Notification
			m.parse()
			s.notifies <- &Notify{m.Pid, m.From, m.Payload}
		}
	}
}
