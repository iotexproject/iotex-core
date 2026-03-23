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
		default:
			w.Header().Set("Allow", "GET, PUT")
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
