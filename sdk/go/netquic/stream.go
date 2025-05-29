package netquic

import (
	"encoding/binary"
	"io"

	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
)

func (c *Client) SendEnvelope(env *axcp.Envelope) error {
	raw, err := axcp.ToBytes(env)
	if err != nil {
		return err
	}
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(raw)))
	if _, err = c.stream.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err = c.stream.Write(raw)
	return err
}

func (c *Client) RecvEnvelope() (*axcp.Envelope, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(c.stream, lenBuf[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(lenBuf[:])
	buf := make([]byte, n)
	if _, err := io.ReadFull(c.stream, buf); err != nil {
		return nil, err
	}
	return axcp.FromBytes(buf)
}
