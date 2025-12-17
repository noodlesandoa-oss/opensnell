/*
 * This file is part of open-snell.
 * open-snell is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 * open-snell is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 * You should have received a copy of the GNU General Public License
 * along with open-snell.  If not, see <https://www.gnu.org/licenses/>.
 */

package snell

import (
	"net"
	"sync"
	"time"
)

type snellFactory = func() (net.Conn, error)

type idleConn struct {
	c net.Conn
	t time.Time
}

type snellPool struct {
	conns   chan *idleConn
	factory snellFactory
	lease   time.Duration
	mu      sync.Mutex
	closed  bool
}

func (p *snellPool) Get() (net.Conn, error) {
	for {
		select {
		case ic := <-p.conns:
			if time.Since(ic.t) > p.lease {
				ic.c.Close()
				continue
			}
			return &snellPoolConn{
				Conn: ic.c,
				pool: p,
				t:    ic.t,
			}, nil
		default:
			c, err := p.factory()
			if err != nil {
				return nil, err
			}
			return &snellPoolConn{
				Conn: c,
				pool: p,
				t:    time.Now(),
			}, nil
		}
	}
}

func (p *snellPool) put(c net.Conn, t time.Time) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		c.Close()
		return
	}
	p.mu.Unlock()

	select {
	case p.conns <- &idleConn{c: c, t: t}:
	default:
		c.Close()
	}
}

func (p *snellPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	close(p.conns)
	for ic := range p.conns {
		ic.c.Close()
	}
}

type snellPoolConn struct {
	net.Conn
	pool *snellPool
	t    time.Time
}

func (pc *snellPoolConn) Close() error {
	if pc.pool == nil {
		return pc.Conn.Close()
	}
	pc.pool.put(pc.Conn, pc.t)
	return nil
}

func (pc *snellPoolConn) MarkUnusable() {
	pc.pool = nil
}

func newSnellPool(maxSize, leaseMS int, factory snellFactory) (*snellPool, error) {
	return &snellPool{
		conns:   make(chan *idleConn, maxSize),
		factory: factory,
		lease:   time.Duration(leaseMS) * time.Millisecond,
	}, nil
}
