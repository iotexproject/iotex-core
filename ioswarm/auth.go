package ioswarm

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	metadataKeyToken   = "x-ioswarm-token"
	metadataKeyAgentID = "x-ioswarm-agent-id"
	apiKeyPrefix       = "iosw_"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const agentIDContextKey contextKey = 0

// DeriveAgentToken generates an API key for the given agent using HMAC-SHA256.
// The key format is "iosw_" + hex(HMAC-SHA256(masterSecret, agentID)).
func DeriveAgentToken(masterSecret, agentID string) string {
	mac := hmac.New(sha256.New, []byte(masterSecret))
	mac.Write([]byte(agentID))
	return apiKeyPrefix + hex.EncodeToString(mac.Sum(nil))
}

// ValidateAgentToken checks that the token matches HMAC-SHA256(masterSecret, agentID).
// Uses constant-time comparison to prevent timing attacks.
func ValidateAgentToken(masterSecret, agentID, token string) bool {
	expected := DeriveAgentToken(masterSecret, agentID)
	return hmac.Equal([]byte(expected), []byte(token))
}

// AgentIDFromContext extracts the verified agent ID from the context.
// Returns empty string if not present (e.g., auth disabled).
func AgentIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(agentIDContextKey).(string); ok {
		return id
	}
	return ""
}

// hmacUnaryInterceptor returns a gRPC unary interceptor that validates
// HMAC-based agent identity from metadata.
func hmacUnaryInterceptor(masterSecret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		newCtx, err := validateHMACFromContext(ctx, masterSecret)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// hmacStreamInterceptor returns a gRPC stream interceptor that validates
// HMAC-based agent identity from metadata.
func hmacStreamInterceptor(masterSecret string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx, err := validateHMACFromContext(ss.Context(), masterSecret)
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: newCtx})
	}
}

// validateHMACFromContext extracts agent_id and token from gRPC metadata,
// validates the HMAC, and returns a new context with the verified agent ID.
func validateHMACFromContext(ctx context.Context, masterSecret string) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	agentIDs := md.Get(metadataKeyAgentID)
	if len(agentIDs) == 0 || agentIDs[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "missing agent ID")
	}

	tokens := md.Get(metadataKeyToken)
	if len(tokens) == 0 || tokens[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "missing auth token")
	}

	agentID := agentIDs[0]
	token := tokens[0]

	if !ValidateAgentToken(masterSecret, agentID, token) {
		return nil, status.Error(codes.Unauthenticated, "invalid auth token")
	}

	return context.WithValue(ctx, agentIDContextKey, agentID), nil
}

// wrappedStream wraps a grpc.ServerStream with a custom context.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// tokenHTTPMiddleware wraps an http.Handler to require HMAC-based auth.
// The header X-Ioswarm-Agent-Id and X-Ioswarm-Token must be present.
// /healthz and /api/stats are always allowed without auth.
func tokenHTTPMiddleware(masterSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/api/stats" || r.URL.Path == "/api/rewards" {
			next.ServeHTTP(w, r)
			return
		}
		agentID := r.Header.Get("X-Ioswarm-Agent-Id")
		token := r.Header.Get("X-Ioswarm-Token")
		if agentID == "" || token == "" || !ValidateAgentToken(masterSecret, agentID, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
