package snell

import (

	"encoding/binary"
	"io"
	"net"
	"testing"
)

func TestWriteHeader_V1(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		err := WriteHeader(client, "example.com", 80, false)
		if err != nil {
			t.Errorf("WriteHeader failed: %v", err)
		}
		client.Close()
	}()

	// Server reads:
	// 1. Version (1)
	// 2. Command (Connect=1)
	// 3. ID Len (0)
	// 4. Host Len
	// 5. Host + Port
	buf := make([]byte, 1024)
	n, err := io.ReadFull(server, buf[:3])
	if err != nil {
		t.Fatalf("Read header start failed: %v", err)
	}
	if n != 3 {
		t.Fatalf("Expected 3 bytes, got %d", n)
	}

	if buf[0] != Version {
		t.Errorf("Expected version %d, got %d", Version, buf[0])
	}
	if buf[1] != CommandConnect {
		t.Errorf("Expected command %d, got %d", CommandConnect, buf[1])
	}
	if buf[2] != 0 {
		t.Errorf("Expected 0 ID length, got %d", buf[2])
	}

	// Read host len
	_, err = io.ReadFull(server, buf[:1])
	hostLen := int(buf[0])
	if hostLen != len("example.com") {
		t.Errorf("Expected host len %d, got %d", len("example.com"), hostLen)
	}

	// Read host + port(2)
	_, err = io.ReadFull(server, buf[:hostLen+2])
	host := string(buf[:hostLen])
	if host != "example.com" {
		t.Errorf("Expected host example.com, got %s", host)
	}

	portBytes := buf[hostLen : hostLen+2]
	port := binary.BigEndian.Uint16(portBytes)
	if port != 80 {
		t.Errorf("Expected port 80, got %d", port)
	}
}

func TestWriteHeader_V2(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		err := WriteHeader(client, "test.local", 443, true)
		if err != nil {
			t.Errorf("WriteHeader failed: %v", err)
		}
		client.Close()
	}()

	buf := make([]byte, 1024)
	// Read Version (1) + Command (1) + IDLen (1)
	_, err := io.ReadFull(server, buf[:3])
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if buf[1] != CommandConnectV2 {
		t.Errorf("Expected ConnectV2 %d, got %d", CommandConnectV2, buf[1])
	}

	// Read HostLen (1)
	_, err = io.ReadFull(server, buf[:1])
	if err != nil {
		t.Fatalf("Read host len failed: %v", err)
	}
	hostLen := int(buf[0])

	// Read Host + Port
	_, err = io.ReadFull(server, buf[:hostLen+2])
	if err != nil {
		t.Fatalf("Read host/port failed: %v", err)
	}
}
