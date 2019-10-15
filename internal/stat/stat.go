/*
Copyright (c) JSC iCore.

This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package stat

import (
	"encoding/json"
	"net/http"

	"github.com/i-core/rlog"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for health checking and versioning.
type Handler struct {
	version string
}

// NewHandler creates a new Handler.
func NewHandler(version string) *Handler {
	return &Handler{version: version}
}

// AddRoutes registers all required routes for the package stat.
func (h *Handler) AddRoutes(apply func(m, p string, h http.Handler, mws ...func(http.Handler) http.Handler)) {
	apply(http.MethodGet, "/health/alive", newHealthHandler())
	apply(http.MethodGet, "/health/ready", newHealthHandler())
	apply(http.MethodGet, "/version", newVersionHandler(h.version))
}

func newHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := rlog.FromContext(r.Context())
		resp := struct {
			Status string `json:"status"`
		}{
			Status: "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Info("Failed to marshal health liveness and readiness status", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

func newVersionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := rlog.FromContext(r.Context())
		resp := struct {
			Version string `json:"version"`
		}{
			Version: version,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Info("Failed to marshal version", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}
