package itx

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/iotexproject/go-pkgs/crypto"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type producerKeysApplyRequest struct {
	PrivateKeys []string `json:"private_keys"`
}

type producerKeysPatchRequest struct {
	AddKeys         []string `json:"add_keys"`
	RemoveAddresses []string `json:"remove_addresses"`
}

type producerKeysResponse struct {
	OperatorAddresses []string `json:"operator_addresses"`
}

// producerKeysMaxBodyBytes bounds the request body size for producer-key
// admin writes. Private keys are 64 hex chars (~66B with quotes), so 1 MiB
// comfortably covers thousands of keys while preventing memory exhaustion.
const producerKeysMaxBodyBytes = 1 << 20

// NewProducerKeysAdmin returns an admin handler for hot-updating producer keys.
func NewProducerKeysAdmin(svr *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizeProducerKeyAdmin(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodGet:
			chainCfg := svr.Config().Chain
			addrs := chainCfg.ProducerAddress()
			addresses := make([]string, 0, len(addrs))
			for _, addr := range addrs {
				addresses = append(addresses, addr.String())
			}
			writeProducerKeyResponse(w, http.StatusOK, producerKeysResponse{
				OperatorAddresses: addresses,
			})
		case http.MethodPut:
			var req producerKeysApplyRequest
			if err := decodeProducerKeyRequest(w, r, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			keys := make([]crypto.PrivateKey, 0, len(req.PrivateKeys))
			for _, encoded := range req.PrivateKeys {
				key, err := crypto.HexStringToPrivateKey(strings.TrimSpace(encoded))
				if err != nil {
					http.Error(w, "invalid private key", http.StatusBadRequest)
					return
				}
				keys = append(keys, key)
			}

			addresses, err := svr.UpdateProducerKeys(keys)
			if err != nil {
				log.L().Error("failed to hot-update producer keys", zap.Error(err))
				http.Error(w, "failed to update producer keys", http.StatusInternalServerError)
				return
			}
			writeProducerKeyResponse(w, http.StatusOK, producerKeysResponse{
				OperatorAddresses: addresses,
			})
		case http.MethodPatch:
			var req producerKeysPatchRequest
			if err := decodeProducerKeyRequest(w, r, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// Build remove set for O(1) lookup.
			removeSet := make(map[string]struct{}, len(req.RemoveAddresses))
			for _, addr := range req.RemoveAddresses {
				removeSet[strings.TrimSpace(addr)] = struct{}{}
			}

			// Retain existing keys in original order, skipping removed addresses.
			chainCfg := svr.Config().Chain
			newKeys := make([]crypto.PrivateKey, 0)
			for _, encoded := range strings.Split(chainCfg.ProducerPrivKey, ",") {
				encoded = strings.TrimSpace(encoded)
				if encoded == "" {
					continue
				}
				key, err := crypto.HexStringToPrivateKey(encoded)
				if err != nil {
					continue
				}
				addr := key.PublicKey().Address().String()
				if _, removed := removeSet[addr]; removed {
					continue
				}
				newKeys = append(newKeys, key)
			}

			// Track addresses already in the list to avoid duplicates when adding.
			existingAddrs := make(map[string]struct{}, len(newKeys))
			for _, key := range newKeys {
				existingAddrs[key.PublicKey().Address().String()] = struct{}{}
			}

			// Append new keys at the end in request order; skip duplicates.
			for _, encoded := range req.AddKeys {
				key, err := crypto.HexStringToPrivateKey(strings.TrimSpace(encoded))
				if err != nil {
					http.Error(w, "invalid private key in add_keys", http.StatusBadRequest)
					return
				}
				addr := key.PublicKey().Address().String()
				if _, exists := existingAddrs[addr]; exists {
					continue // already present, skip
				}
				newKeys = append(newKeys, key)
				existingAddrs[addr] = struct{}{}
			}

			if len(newKeys) == 0 {
				http.Error(w, "cannot remove all producer keys", http.StatusBadRequest)
				return
			}

			addresses, err := svr.UpdateProducerKeys(newKeys)
			if err != nil {
				log.L().Error("failed to patch producer keys", zap.Error(err))
				http.Error(w, "failed to update producer keys", http.StatusInternalServerError)
				return
			}
			writeProducerKeyResponse(w, http.StatusOK, producerKeysResponse{
				OperatorAddresses: addresses,
			})
		default:
			w.Header().Set("Allow", "GET, PUT, PATCH")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// authorizeProducerKeyAdmin requires IOTEX_ADMIN_TOKEN to be set and matched
// via either an "Authorization: Bearer <token>" or "X-Admin-Token: <token>"
// header. The endpoint accepts raw producer private keys, so an unset token
// fails closed to prevent unauthenticated key rotation if the admin port is
// reachable.
func authorizeProducerKeyAdmin(r *http.Request) bool {
	token := strings.TrimSpace(os.Getenv("IOTEX_ADMIN_TOKEN"))
	if token == "" {
		return false
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") && strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")) == token {
		return true
	}

	return strings.TrimSpace(r.Header.Get("X-Admin-Token")) == token
}

// decodeProducerKeyRequest caps the request body and rejects unknown fields,
// guarding the admin endpoint against arbitrarily large bodies and silent
// typos in client payloads.
func decodeProducerKeyRequest(w http.ResponseWriter, r *http.Request, v interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, producerKeysMaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.New("invalid json body")
	}
	return nil
}

func writeProducerKeyResponse(w http.ResponseWriter, status int, payload producerKeysResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
