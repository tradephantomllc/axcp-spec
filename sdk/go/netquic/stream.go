package netquic

import (
	"encoding/binary"
	"github.com/tradephantomllc/axcp-spec/sdk/go/axcp"
)

func (c *Client) SendEnvelope(env *axcp.Envelope) error {
	if c == nil || c.stream == nil {
		return ErrNotConnected
	}
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()
	raw, err := axcp.ToBytes(env)
	if err != nil {
		return err
	}
	return writeFrame(c.stream, raw, binary.LittleEndian, true)
}

func (c *Client) RecvEnvelope() (*axcp.Envelope, error) {
	if c == nil || c.stream == nil {
		return nil, ErrNotConnected
	}
	c.recvMutex.Lock()
	defer c.recvMutex.Unlock()
	data, err := readFrame(c.stream, binary.LittleEndian, true)
	if err != nil {
		return nil, err
	}
	return axcp.FromBytes(data)
}
