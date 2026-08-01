package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"equipment-telemetry-simulator/internal/model"
	"equipment-telemetry-simulator/internal/simulator"
)

type API struct {
	manager *simulator.Manager
	logger  *slog.Logger
}

func NewAPI(manager *simulator.Manager, logger *slog.Logger) *API {
	if logger == nil {
		logger = slog.Default()
	}
	return &API{manager: manager, logger: logger}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("OPTIONS /", a.options)
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/v1/assets", a.listAssets)
	mux.HandleFunc("POST /api/v1/assets", a.registerAsset)
	mux.HandleFunc("GET /api/v1/assets/{assetId}", a.getAsset)
	mux.HandleFunc("PUT /api/v1/assets/{assetId}", a.replaceAsset)
	mux.HandleFunc("PATCH /api/v1/assets/{assetId}", a.patchAsset)
	mux.HandleFunc("DELETE /api/v1/assets/{assetId}", a.deleteAsset)
	mux.HandleFunc("GET /api/v1/telemetry", a.listTelemetry)
	mux.HandleFunc("GET /api/v1/telemetry/{assetId}", a.getTelemetryByPath)
	mux.HandleFunc("GET /api/v1/faults", a.listFaults)
	mux.HandleFunc("POST /api/v1/faults", a.injectFault)
	mux.HandleFunc("PUT /api/v1/faults/{assetId}", a.replaceFaults)
	mux.HandleFunc("PATCH /api/v1/faults/{assetId}", a.patchFaults)
	mux.HandleFunc("DELETE /api/v1/faults/{assetId}", a.deleteFaults)
	mux.HandleFunc("DELETE /api/v1/faults/{assetId}/{faultType}", a.deleteFault)
	mux.HandleFunc("POST /api/v1/faults/inject", a.injectFault)
	mux.HandleFunc("POST /api/v1/faults/clear", a.clearFaults)

	return recoverMiddleware(corsMiddleware(loggingMiddleware(a.logger, mux)))
}

func (a *API) options(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) listAssets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.manager.ListAssets())
}

func (a *API) getAsset(w http.ResponseWriter, r *http.Request) {
	asset, err := a.manager.GetAsset(r.PathValue("assetId"))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) replaceAsset(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateAssetRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	asset, err := a.manager.ReplaceAsset(r.PathValue("assetId"), req)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) patchAsset(w http.ResponseWriter, r *http.Request) {
	var req model.PatchAssetRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	asset, err := a.manager.PatchAsset(r.PathValue("assetId"), req)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) deleteAsset(w http.ResponseWriter, r *http.Request) {
	if err := a.manager.DeleteAsset(r.PathValue("assetId")); err != nil {
		writeManagerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("assetId") != "" {
		a.getTelemetry(w, r)
		return
	}
	writeJSON(w, http.StatusOK, a.manager.ListAssets())
}

func (a *API) getTelemetry(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("assetId")
	asset, err := a.manager.GetTelemetry(assetID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) getTelemetryByPath(w http.ResponseWriter, r *http.Request) {
	asset, err := a.manager.GetTelemetry(r.PathValue("assetId"))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) registerAsset(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterAssetRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	asset, err := a.manager.RegisterAsset(req.AssetID, req.AssetType)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (a *API) injectFault(w http.ResponseWriter, r *http.Request) {
	var req model.InjectFaultRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	asset, err := a.manager.InjectFault(req.AssetID, req.FaultType)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) listFaults(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("assetId")
	faults, err := a.manager.ListFaults(assetID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"assetId":      assetID,
		"activeFaults": faults,
	})
}

func (a *API) replaceFaults(w http.ResponseWriter, r *http.Request) {
	var req model.ReplaceFaultsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	assetID := r.PathValue("assetId")
	if req.AssetID != "" && req.AssetID != assetID {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "path assetId does not match body assetId"})
		return
	}

	asset, err := a.manager.ReplaceFaults(assetID, req.FaultTypes)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) patchFaults(w http.ResponseWriter, r *http.Request) {
	var req model.PatchFaultsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	assetID := r.PathValue("assetId")
	if req.AssetID != "" && req.AssetID != assetID {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "path assetId does not match body assetId"})
		return
	}

	asset, err := a.manager.PatchFaults(assetID, req.Add, req.Remove)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) deleteFaults(w http.ResponseWriter, r *http.Request) {
	asset, err := a.manager.ClearFaults(r.PathValue("assetId"))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) deleteFault(w http.ResponseWriter, r *http.Request) {
	asset, err := a.manager.DeleteFault(r.PathValue("assetId"), model.FaultType(r.PathValue("faultType")))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func (a *API) clearFaults(w http.ResponseWriter, r *http.Request) {
	var req model.ClearFaultRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	asset, err := a.manager.ClearFaults(req.AssetID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid JSON body: " + err.Error()})
		return false
	}
	return true
}

func writeManagerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, simulator.ErrMissingIdentifier), errors.Is(err, simulator.ErrInvalidRequest):
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
	case errors.Is(err, simulator.ErrAssetNotFound):
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
	case errors.Is(err, simulator.ErrAssetExists):
		writeJSON(w, http.StatusConflict, model.ErrorResponse{Error: err.Error()})
	case errors.Is(err, simulator.ErrUnsupportedAsset), errors.Is(err, simulator.ErrUnsupportedFault), errors.Is(err, simulator.ErrUnsupportedStatus):
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Info("request completed", "method", r.Method, "path", r.URL.Path, "status", recorder.status)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
