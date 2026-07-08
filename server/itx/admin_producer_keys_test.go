// Copyright (c) 2025 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package itx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthorizeProducerKeyAdmin(t *testing.T) {
	cases := []struct {
		name    string
		token   string
		headers map[string]string
		want    bool
	}{
		{name: "token unset rejects request", token: "", want: false},
		{
			name:    "token unset rejects even with header",
			token:   "",
			headers: map[string]string{"X-Admin-Token": "anything"},
			want:    false,
		},
		{name: "token set but no header rejects", token: "secret", want: false},
		{
			name:    "wrong bearer rejects",
			token:   "secret",
			headers: map[string]string{"Authorization": "Bearer wrong"},
			want:    false,
		},
		{
			name:    "matching bearer accepts",
			token:   "secret",
			headers: map[string]string{"Authorization": "Bearer secret"},
			want:    true,
		},
		{
			name:    "matching X-Admin-Token accepts",
			token:   "secret",
			headers: map[string]string{"X-Admin-Token": "secret"},
			want:    true,
		},
		{
			name:    "wrong X-Admin-Token rejects",
			token:   "secret",
			headers: map[string]string{"X-Admin-Token": "wrong"},
			want:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("IOTEX_ADMIN_TOKEN", tc.token)
			r := httptest.NewRequest(http.MethodGet, "/producer-keys", nil)
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}
			require.Equal(t, tc.want, authorizeProducerKeyAdmin(r))
		})
	}
}

func TestDecodeProducerKeyRequest(t *testing.T) {
	t.Run("valid payload decodes", func(t *testing.T) {
		body := bytes.NewBufferString(`{"private_keys":["a","b"]}`)
		r := httptest.NewRequest(http.MethodPut, "/producer-keys", body)
		w := httptest.NewRecorder()
		var got producerKeysApplyRequest
		require.NoError(t, decodeProducerKeyRequest(w, r, &got))
		require.Equal(t, []string{"a", "b"}, got.PrivateKeys)
	})

	t.Run("malformed json rejected", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPut, "/producer-keys", bytes.NewBufferString(`{"private_keys"`))
		w := httptest.NewRecorder()
		var got producerKeysApplyRequest
		require.Error(t, decodeProducerKeyRequest(w, r, &got))
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		body := bytes.NewBufferString(`{"private_keys":["a"],"extra":"x"}`)
		r := httptest.NewRequest(http.MethodPut, "/producer-keys", body)
		w := httptest.NewRecorder()
		var got producerKeysApplyRequest
		require.Error(t, decodeProducerKeyRequest(w, r, &got))
	})

	t.Run("body over max size rejected", func(t *testing.T) {
		// One byte over the cap.
		oversized := strings.Repeat("a", producerKeysMaxBodyBytes+1)
		r := httptest.NewRequest(http.MethodPut, "/producer-keys", bytes.NewBufferString(oversized))
		w := httptest.NewRecorder()
		var got producerKeysApplyRequest
		require.Error(t, decodeProducerKeyRequest(w, r, &got))
	})
}

func TestProducerKeysAdminHandler_Unauthorized(t *testing.T) {
	t.Setenv("IOTEX_ADMIN_TOKEN", "")
	handler := NewProducerKeysAdmin(nil)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/producer-keys", bytes.NewBufferString("{}"))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestProducerKeysAdminHandler_MethodNotAllowed(t *testing.T) {
	t.Setenv("IOTEX_ADMIN_TOKEN", "secret")
	handler := NewProducerKeysAdmin(nil)
	r := httptest.NewRequest(http.MethodPost, "/producer-keys", bytes.NewBufferString("{}"))
	r.Header.Set("X-Admin-Token", "secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
	require.Equal(t, "GET, PUT, PATCH", w.Header().Get("Allow"))
}

func TestProducerKeysResponseRoundTrip(t *testing.T) {
	w := httptest.NewRecorder()
	writeProducerKeyResponse(w, http.StatusOK, producerKeysResponse{
		OperatorAddresses: []string{"io1abc", "io1def"},
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var got producerKeysResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, []string{"io1abc", "io1def"}, got.OperatorAddresses)
}
