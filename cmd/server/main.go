package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"equipment-telemetry-simulator/internal/handler"
	"equipment-telemetry-simulator/internal/model"
	"equipment-telemetry-simulator/internal/simulator"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	tickInterval := durationFromEnv("TICK_INTERVAL", 2*time.Second)
	pushInterval := durationFromEnv("PUSH_INTERVAL", tickInterval)

	var pushClient *simulator.PushClient
	if boolFromEnv("PUSH_MODE") {
		targetURL := strings.TrimSpace(os.Getenv("TARGET_TOIR_URL"))
		if targetURL == "" {
			logger.Error("PUSH_MODE=true requires TARGET_TOIR_URL")
			os.Exit(1)
		}
		pushClient = simulator.NewPushClient(targetURL)
	}

	manager := simulator.NewManager(simulator.Config{
		TickInterval: tickInterval,
		PushClient:   pushClient,
		PushInterval: pushInterval,
		Logger:       logger,
	})
	registerSeedAssets(manager, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	manager.Start(ctx)

	api := handler.NewAPI(manager, logger)
	server := &http.Server{
		Addr:              ":" + stringFromEnv("PORT", "8080"),
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("equipment telemetry simulator listening", "addr", server.Addr, "tickInterval", tickInterval.String())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}

func registerSeedAssets(manager *simulator.Manager, logger *slog.Logger) {
	seedAssets := []struct {
		id        string
		assetType model.AssetType
	}{
		{id: "PUMP-101", assetType: model.AssetTypeWaterPump},
		{id: "PUMP-102", assetType: model.AssetTypeWaterPump},
		{id: "COMP-301", assetType: model.AssetTypeAirCompressor},
		{id: "GEN-401", assetType: model.AssetTypeDieselGenerator},
		{id: "TRUCK-501", assetType: model.AssetTypeHeavyTruck},
	}

	for _, asset := range seedAssets {
		if _, err := manager.RegisterAsset(asset.id, asset.assetType); err != nil {
			logger.Warn("seed asset registration skipped", "assetId", asset.id, "error", err)
		}
	}
}

func stringFromEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func boolFromEnv(key string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}
