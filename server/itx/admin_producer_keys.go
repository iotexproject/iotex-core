package itx

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"go.uber.org/zap"
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

// NewProducerKeysAdmin returns an admin handler for hot-updating producer keys.
func NewProducerKeysAdmin(svr *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizeProducerKeyAdmin(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodGet:
			cfg := svr.Config()
			addrs := cfg.Chain.ProducerAddress()
			addresses := make([]string, 0, len(addrs))
			for _, addr := range addrs {
				addresses = append(addresses, addr.String())
			}
			writeProducerKeyResponse(w, http.StatusOK, producerKeysResponse{
				OperatorAddresses: addresses,
			})
		case http.MethodPut:
			var req producerKeysApplyRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
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
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}

			// Build current key map (address → key) from in-memory config.
			cfg := svr.Config()
			currentKeys := make(map[string]crypto.PrivateKey)
			for _, encoded := range strings.Split(cfg.Chain.ProducerPrivKey, ",") {
				encoded = strings.TrimSpace(encoded)
				if encoded == "" {
					continue
				}
				key, err := crypto.HexStringToPrivateKey(encoded)
				if err != nil {
					continue
				}
				currentKeys[key.PublicKey().Address().String()] = key
			}

			// Add new keys.
			for _, encoded := range req.AddKeys {
				key, err := crypto.HexStringToPrivateKey(strings.TrimSpace(encoded))
				if err != nil {
					http.Error(w, "invalid private key in add_keys", http.StatusBadRequest)
					return
				}
				currentKeys[key.PublicKey().Address().String()] = key
			}

			// Remove by address.
			for _, addr := range req.RemoveAddresses {
				delete(currentKeys, strings.TrimSpace(addr))
			}

			if len(currentKeys) == 0 {
				http.Error(w, "cannot remove all producer keys", http.StatusBadRequest)
				return
			}

			newKeys := make([]crypto.PrivateKey, 0, len(currentKeys))
			for _, key := range currentKeys {
				newKeys = append(newKeys, key)
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

func authorizeProducerKeyAdmin(r *http.Request) bool {
	token := strings.TrimSpace(os.Getenv("IOTEX_ADMIN_TOKEN"))
	if token == "" {
		return true
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") && strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")) == token {
		return true
	}

	return strings.TrimSpace(r.Header.Get("X-Admin-Token")) == token
}

func writeProducerKeyResponse(w http.ResponseWriter, status int, payload producerKeysResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
