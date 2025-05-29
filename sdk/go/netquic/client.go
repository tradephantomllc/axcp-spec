package netquic

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/quic-go/quic-go"
)

const defaultTimeout = 8 * time.Second

type Client struct {
	conn   quic.Connection
	stream quic.Stream
}

func Dial(addr string, tlsConf *tls.Config) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	conn, err := quic.DialAddr(ctx, addr, tlsConf, nil)
	if err != nil {
		return nil, err
	}
	str, err := conn.OpenStreamSync(ctx)
	if err != nil {
		conn.CloseWithError(0, "stream open fail")
		return nil, err
	}
	return &Client{conn: conn, stream: str}, nil
}

func (c *Client) Close() error { return c.conn.CloseWithError(0, "") }
