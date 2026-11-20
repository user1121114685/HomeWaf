package wafpipeline

import (
	"net"
	"sync"
	"sync/atomic"

	"golang.org/x/net/netutil"
)

const totalConnectionLimit = 8192

var activePipelineConnections atomic.Int64
var rejectedPipelineConnections atomic.Uint64

type limitedPipelineListener struct{ net.Listener }
type limitedPipelineConnection struct {
	net.Conn
	once sync.Once
}

func (l *limitedPipelineListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		reserved := false
		for {
			current := activePipelineConnections.Load()
			if current >= totalConnectionLimit {
				break
			}
			if activePipelineConnections.CompareAndSwap(current, current+1) {
				reserved = true
				break
			}
		}
		if reserved {
			return &limitedPipelineConnection{Conn: c}, nil
		}
		rejectedPipelineConnections.Add(1)
		c.Close()
	}
}
func (c *limitedPipelineConnection) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { activePipelineConnections.Add(-1) })
	return err
}
func limitPipelineListener(l net.Listener, maxInFlight int) net.Listener {
	return &limitedPipelineListener{Listener: netutil.LimitListener(l, positive(maxInFlight, 32)*4)}
}
