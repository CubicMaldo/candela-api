package dos_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestSlowloris_ReadHeaderTimeout verifies that slowloris-style slow HTTP clients
// (sending headers slowly byte-by-byte) are forcibly terminated by ReadHeaderTimeout,
// preventing goroutine and file descriptor exhaustion, while normal clients are unharmed.
func TestSlowloris_ReadHeaderTimeout(t *testing.T) {
	router := gin.New()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "received"})
	})

	// Bind to an arbitrary available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Configure server with a strict ReadHeaderTimeout (250ms for the test)
	srv := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 250 * time.Millisecond,
		ReadTimeout:       500 * time.Millisecond,
	}

	go func() {
		_ = srv.Serve(listener)
	}()
	defer func() {
		_ = srv.Close()
	}()

	// 1. Slowloris attack simulation:
	// Connect via raw TCP and send headers extremely slowly (1 header line every 150ms)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("failed to dial server: %v", err)
	}
	defer conn.Close()

	// Send partial request line
	_, _ = conn.Write([]byte("POST /api/test HTTP/1.1\r\n"))
	_, _ = conn.Write([]byte(fmt.Sprintf("Host: %s\r\n", serverAddr)))

	// Sleep longer than ReadHeaderTimeout before finishing the headers
	time.Sleep(400 * time.Millisecond)

	// Attempt to send another header line; the server must have closed or refused the socket
	_, writeErr := conn.Write([]byte("X-Slow-Header: attacking\r\n\r\n"))

	// Read response or EOF
	reader := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	respLine, readErr := reader.ReadString('\n')

	// Either the write failed or the server returned 408 Request Timeout / EOF / reset
	isTerminated := writeErr != nil || readErr != nil || strings.Contains(respLine, "408") || respLine == ""
	if !isTerminated {
		t.Fatalf("expected server to terminate slow connection, but got response: %s", respLine)
	}

	// 2. Legitimate request must still succeed promptly
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/health", serverAddr))
	if err != nil {
		t.Fatalf("legitimate request to /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /health, got %d", resp.StatusCode)
	}
}
