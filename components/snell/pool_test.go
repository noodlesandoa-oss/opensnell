package snell

import (
	"errors"
	"net"
	"testing"
	"time"
)

// MockConn implements net.Conn for testing
type MockConn struct {
	closed bool
}

func (m *MockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *MockConn) Write(b []byte) (n int, err error)  { return 0, nil }
func (m *MockConn) Close() error                       { m.closed = true; return nil }
func (m *MockConn) LocalAddr() net.Addr                { return nil }
func (m *MockConn) RemoteAddr() net.Addr               { return nil }
func (m *MockConn) SetDeadline(t time.Time) error      { return nil }
func (m *MockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestSnellPool_GetPut(t *testing.T) {
	factory := func() (net.Conn, error) {
		return &MockConn{}, nil
	}

	// Create pool with size 2, lease 100ms
	pool, err := newSnellPool(2, 100, factory)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// 1. Get a connection
	conn1, err := pool.Get()
	if err != nil {
		t.Fatalf("Failed to get conn1: %v", err)
	}

	// 2. Put it back
	conn1.Close() // This calls snellPoolConn.Close -> pool.put

	// 3. Get it again (should be the same one if we could check identity, but at least we get one)
	conn2, err := pool.Get()
	if err != nil {
		t.Fatalf("Failed to get conn2: %v", err)
	}
	conn2.Close()
}

func TestSnellPool_LeaseExpiration(t *testing.T) {
	factory := func() (net.Conn, error) {
		return &MockConn{}, nil
	}

	// Lease 50ms
	pool, _ := newSnellPool(1, 50, factory)
	defer pool.Close()

	conn, _ := pool.Get()
	// Cast to verify it's our wrapper
	pc, ok := conn.(*snellPoolConn)
	if !ok {
		t.Fatalf("Expected snellPoolConn")
	}
	
	// Manually set time to past to simulate expiration
	// Since we can't easily mock time.Now() in the pool without dependency injection,
	// we will wait. 50ms is short enough.
	
	conn.Close() // Return to pool

	time.Sleep(100 * time.Millisecond) // Wait for lease to expire

	conn2, _ := pool.Get()
	pc2, _ := conn2.(*snellPoolConn)

	// If logic works, conn2 should be a NEW connection, not the old one.
	// But since we can't easily check pointer equality of the underlying MockConn without exposing it,
	// we rely on the fact that the pool loop would have closed the old one and created a new one.
	// We can check if the old MockConn is closed.
	
	oldMock := pc.Conn.(*MockConn)
	if !oldMock.closed {
		t.Errorf("Old connection should have been closed due to lease expiration")
	}
	
	pc2.Close()
}

func TestSnellPool_Capacity(t *testing.T) {
	factory := func() (net.Conn, error) {
		return &MockConn{}, nil
	}

	// Capacity 1
	pool, _ := newSnellPool(1, 1000, factory)
	defer pool.Close()

	c1, _ := pool.Get()
	c2, _ := pool.Get()

	c1.Close() // Should go to pool
	c2.Close() // Should be discarded/closed because pool is full

	// Verify c2 is closed
	mc2 := c2.(*snellPoolConn).Conn.(*MockConn)
	// Wait a tiny bit for channel select to process if it was async (it's not, but good practice)
	time.Sleep(10 * time.Millisecond)
	
	if !mc2.closed {
		// In our implementation:
		// select { case p.conns <- ...: default: c.Close() }
		// So if channel is full, it closes immediately.
		t.Errorf("Connection exceeding capacity should be closed")
	}
}

func TestSnellPool_FactoryError(t *testing.T) {
	errFactory := errors.New("factory failed")
	factory := func() (net.Conn, error) {
		return nil, errFactory
	}

	pool, _ := newSnellPool(1, 1000, factory)
	defer pool.Close()

	_, err := pool.Get()
	if err != errFactory {
		t.Errorf("Expected factory error, got %v", err)
	}
}
