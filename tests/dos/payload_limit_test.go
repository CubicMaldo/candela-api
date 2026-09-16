package dos_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tuusuario/candela-api/internal/web/middleware"
)

// infiniteReader produces an endless stream of bytes to simulate a streaming payload bomb.
type infiniteReader struct{}

func (i *infiniteReader) Read(p []byte) (n int, err error) {
	for j := range p {
		p[j] = 'A'
	}
	return len(p), nil
}

func TestPayloadLimit_ContentLengthHeaderExceeded(t *testing.T) {
	router := gin.New()
	// Set 1 MB limit
	router.Use(middleware.MaxBodySize(1 << 20))
	router.POST("/api/test-json", func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 1. Send request declaring Content-Length of 10MB (10 * 1024 * 1024)
	req := httptest.NewRequest(http.MethodPost, "/api/test-json", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 10 * 1024 * 1024 // 10MB declared

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 StatusRequestEntityTooLarge, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPayloadLimit_StreamingPayloadBomb(t *testing.T) {
	router := gin.New()
	limit := int64(1 << 20) // 1 MB
	router.Use(middleware.MaxBodySize(limit))
	router.POST("/api/test-upload", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) || strings.Contains(err.Error(), "request body too large") {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "body too large"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 2. Stream 3 MB without Content-Length header (chunked / streaming bomb)
	bomb := io.LimitReader(&infiniteReader{}, 3*1024*1024)
	req := httptest.NewRequest(http.MethodPost, "/api/test-upload", bomb)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = -1 // Chunked / unknown length

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 StatusRequestEntityTooLarge on streaming bomb, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPayloadLimit_ValidPayloadSucceeds(t *testing.T) {
	router := gin.New()
	router.Use(middleware.MaxBodySize(1 << 20))
	router.POST("/api/test-json", func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": body["message"]})
	})

	payload := `{"message":"hello world within 1mb"}`
	req := httptest.NewRequest(http.MethodPost, "/api/test-json", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(payload))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid payload, got %d: %s", w.Code, w.Body.String())
	}
}
