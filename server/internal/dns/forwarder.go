package dns

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Upstream struct {
	Addr    string
	Timeout time.Duration
}

type PooledForwarder struct {
	upstreams []Upstream
	pool      sync.Pool
	mu        sync.RWMutex
	healthy   map[int]bool
}

func NewPooledForwarder(upstreams []Upstream) *PooledForwarder {
	f := &PooledForwarder{
		upstreams: upstreams,
		healthy:   make(map[int]bool),
	}
	f.pool.New = func() any {
		buf := make([]byte, 1500)
		return &buf
	}

	for i := range upstreams {
		f.healthy[i] = true
	}

	if len(f.upstreams) == 0 {
		f.upstreams = []Upstream{{Addr: "1.1.1.1:53", Timeout: 3 * time.Second}}
		f.healthy[0] = true
	}

	go f.healthLoop()
	return f
}

func (f *PooledForwarder) Forward(data []byte) ([]byte, error) {
	bufPtr := f.pool.Get().(*[]byte)
	defer f.pool.Put(bufPtr)
	buf := *bufPtr

	var lastErr error
	for i := range f.upstreams {
		f.mu.RLock()
		ok := f.healthy[i]
		f.mu.RUnlock()
		if !ok {
			continue
		}

		resp, err := f.forwardTo(i, data, buf)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		f.mu.Lock()
		f.healthy[i] = false
		f.mu.Unlock()
	}

	return nil, fmt.Errorf("all upstreams failed: %w", lastErr)
}

func (f *PooledForwarder) forwardTo(idx int, data, buf []byte) ([]byte, error) {
	up := f.upstreams[idx]
	conn, err := net.DialTimeout("udp", up.Addr, up.Timeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", up.Addr, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(up.Timeout)); err != nil {
		return nil, err
	}

	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("write %s: %w", up.Addr, err)
	}

	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", up.Addr, err)
	}

	resp := make([]byte, n)
	copy(resp, buf[:n])
	return resp, nil
}

func (f *PooledForwarder) healthLoop() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		for i, up := range f.upstreams {
			conn, err := net.DialTimeout("udp", up.Addr, 2*time.Second)
			if err != nil {
				f.mu.Lock()
				f.healthy[i] = false
				f.mu.Unlock()
				continue
			}
			conn.Close()
			f.mu.Lock()
			f.healthy[i] = true
			f.mu.Unlock()
		}
	}
}
