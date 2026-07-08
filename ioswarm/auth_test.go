package ioswarm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testMasterSecret = "test-secret-ioswarm-2026"

func TestDeriveAgentToken(t *testing.T) {
	token := DeriveAgentToken(testMasterSecret, "ant-x7k2")
	if !strings.HasPrefix(token, apiKeyPrefix) {
		t.Fatalf("expected prefix %q, got %q", apiKeyPrefix, token[:5])
	}
	// HMAC output is 32 bytes = 64 hex chars + prefix
	if len(token) != len(apiKeyPrefix)+64 {
		t.Fatalf("expected length %d, got %d", len(apiKeyPrefix)+64, len(token))
	}

	// Same inputs produce same output (deterministic)
	token2 := DeriveAgentToken(testMasterSecret, "ant-x7k2")
	if token != token2 {
		t.Fatal("DeriveAgentToken is not deterministic")
	}

	// Different agent ID produces different token
	token3 := DeriveAgentToken(testMasterSecret, "ant-y9z3")
	if token == token3 {
		t.Fatal("different agents should have different tokens")
	}
}

func TestValidateAgentToken(t *testing.T) {
	agentID := "ant-001"
	token := DeriveAgentToken(testMasterSecret, agentID)

	if !ValidateAgentToken(testMasterSecret, agentID, token) {
		t.Fatal("valid token should pass validation")
	}

	// Wrong agent ID
	if ValidateAgentToken(testMasterSecret, "ant-002", token) {
		t.Fatal("wrong agent ID should fail validation")
	}

	// Wrong secret
	if ValidateAgentToken("wrong-secret", agentID, token) {
		t.Fatal("wrong secret should fail validation")
	}

	// Tampered token
	if ValidateAgentToken(testMasterSecret, agentID, token+"x") {
		t.Fatal("tampered token should fail validation")
	}

	// Empty token
	if ValidateAgentToken(testMasterSecret, agentID, "") {
		t.Fatal("empty token should fail validation")
	}
}

func TestHMACInterceptorValid(t *testing.T) {
	agentID := "ant-001"
	token := DeriveAgentToken(testMasterSecret, agentID)

	md := metadata.New(map[string]string{
		metadataKeyAgentID: agentID,
		metadataKeyToken:   token,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	interceptor := hmacUnaryInterceptor(testMasterSecret)
	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		// Verify agent ID is in context
		if got := AgentIDFromContext(ctx); got != agentID {
			t.Fatalf("expected agent ID %q in context, got %q", agentID, got)
		}
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestHMACInterceptorWrongAgent(t *testing.T) {
	// Token derived for ant-001, but metadata claims ant-002
	token := DeriveAgentToken(testMasterSecret, "ant-001")

	md := metadata.New(map[string]string{
		metadataKeyAgentID: "ant-002",
		metadataKeyToken:   token,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	interceptor := hmacUnaryInterceptor(testMasterSecret)
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})

	if err == nil {
		t.Fatal("expected error for mismatched agent")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestHMACInterceptorMissingMetadata(t *testing.T) {
	interceptor := hmacUnaryInterceptor(testMasterSecret)

	// No metadata at all
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error for missing metadata")
	}

	// Missing agent ID
	md := metadata.New(map[string]string{metadataKeyToken: "sometoken"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error for missing agent ID")
	}

	// Missing token
	md = metadata.New(map[string]string{metadataKeyAgentID: "ant-001"})
	ctx = metadata.NewIncomingContext(context.Background(), md)
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestHMACStreamInterceptorValid(t *testing.T) {
	agentID := "ant-stream-1"
	token := DeriveAgentToken(testMasterSecret, agentID)

	md := metadata.New(map[string]string{
		metadataKeyAgentID: agentID,
		metadataKeyToken:   token,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	interceptor := hmacStreamInterceptor(testMasterSecret)
	called := false
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		called = true
		if got := AgentIDFromContext(stream.Context()); got != agentID {
			t.Fatalf("expected agent ID %q in stream context, got %q", agentID, got)
		}
		return nil
	}

	mockStream := &mockServerStream{ctx: ctx}
	err := interceptor(nil, mockStream, &grpc.StreamServerInfo{}, handler)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestAgentIDFromContextEmpty(t *testing.T) {
	if id := AgentIDFromContext(context.Background()); id != "" {
		t.Fatalf("expected empty, got %q", id)
	}
}

func TestHTTPMiddlewareAllowsHealthz(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := tokenHTTPMiddleware(testMasterSecret, inner)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /healthz, got %d", w.Code)
	}
}

func TestHTTPMiddlewareBlocksWithoutToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := tokenHTTPMiddleware(testMasterSecret, inner)

	req := httptest.NewRequest("GET", "/swarm/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHTTPMiddlewarePassesWithValidHMAC(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := tokenHTTPMiddleware(testMasterSecret, inner)

	agentID := "ant-http"
	token := DeriveAgentToken(testMasterSecret, agentID)

	req := httptest.NewRequest("GET", "/swarm/status", nil)
	req.Header.Set("X-Ioswarm-Agent-Id", agentID)
	req.Header.Set("X-Ioswarm-Token", token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHTTPMiddlewareRejectsWrongAgent(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := tokenHTTPMiddleware(testMasterSecret, inner)

	token := DeriveAgentToken(testMasterSecret, "ant-real")

	req := httptest.NewRequest("GET", "/swarm/status", nil)
	req.Header.Set("X-Ioswarm-Agent-Id", "ant-fake")
	req.Header.Set("X-Ioswarm-Token", token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// mockServerStream implements grpc.ServerStream for testing stream interceptors.
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}
