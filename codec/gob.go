package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// GobCodec 是针对Gob编码报文处理的一套流程
type GobCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	enc  *gob.Encoder
	dec  *gob.Decoder
}

var _ Codec = (*GobCodec)(nil)

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		enc:  gob.NewEncoder(conn),
		dec:  gob.NewDecoder(conn),
	}
}

func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)

}

func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *GobCodec) Writer(h *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()

	if err := c.enc.Encode(h); err != nil {
		log.Println("gob encode header error:", err)
		return err
	}

	if err := c.enc.Encode(body); err != nil {
		log.Println("gob encode body error:", err)
		return err
	}

	return nil
}

func (c *GobCodec) Close() error {
	return c.conn.Close()
}
