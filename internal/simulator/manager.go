package simulator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"time"

	"equipment-telemetry-simulator/internal/model"
)

var (
	ErrAssetExists       = errors.New("asset already exists")
	ErrAssetNotFound     = errors.New("asset not found")
	ErrUnsupportedFault  = errors.New("unsupported fault type")
	ErrUnsupportedAsset  = errors.New("unsupported asset type")
	ErrUnsupportedStatus = errors.New("unsupported asset status")
	ErrMissingIdentifier = errors.New("assetId is required")
	ErrInvalidRequest    = errors.New("invalid request")
)

type Manager struct {
	mu          sync.RWMutex
	assets      map[string]*model.Asset
	tick        time.Duration
	rng         *rand.Rand
	logger      *slog.Logger
	pushClient  *PushClient
	pushEvery   time.Duration
	roundDigits int
}

type Config struct {
	TickInterval time.Duration
	PushClient   *PushClient
	PushInterval time.Duration
	Logger       *slog.Logger
}

func NewManager(cfg Config) *Manager {
	tick := cfg.TickInterval
	if tick <= 0 {
		tick = 2 * time.Second
	}
	pushEvery := cfg.PushInterval
	if pushEvery <= 0 {
		pushEvery = tick
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Manager{
		assets:      make(map[string]*model.Asset),
		tick:        tick,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		logger:      logger,
		pushClient:  cfg.PushClient,
		pushEvery:   pushEvery,
		roundDigits: 2,
	}
}

func (m *Manager) Start(ctx context.Context) {
	go m.runTicker(ctx)
	if m.pushClient != nil {
		go m.runPusher(ctx)
	}
}

func (m *Manager) RegisterAsset(assetID string, assetType model.AssetType) (model.Asset, error) {
	if assetID == "" {
		return model.Asset{}, ErrMissingIdentifier
	}
	template, err := TemplateFor(assetType)
	if err != nil {
		return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedAsset, assetType)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.assets[assetID]; exists {
		return model.Asset{}, ErrAssetExists
	}

	asset := &model.Asset{
		AssetID:      assetID,
		AssetType:    assetType,
		Status:       model.AssetStatusRunning,
		Metrics:      make(map[string]model.MetricValue, len(template.Metrics)),
		ActiveFaults: []model.FaultType{},
		UpdatedAt:    time.Now().UTC(),
	}
	for _, metric := range template.Metrics {
		value := m.randomBetween(metric.Min, metric.Max)
		if metric.Sticky {
			value = metric.Max
		}
		asset.Metrics[metric.Name] = model.MetricValue{Value: m.round(value), Unit: metric.Unit}
	}

	m.assets[assetID] = asset
	return cloneAsset(asset), nil
}

func (m *Manager) ListAssets() []model.Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assets := make([]model.Asset, 0, len(m.assets))
	for _, asset := range m.assets {
		assets = append(assets, cloneAsset(asset))
	}
	return assets
}

func (m *Manager) GetAsset(assetID string) (model.Asset, error) {
	return m.GetTelemetry(assetID)
}

func (m *Manager) GetTelemetry(assetID string) (model.Asset, error) {
	if assetID == "" {
		return model.Asset{}, ErrMissingIdentifier
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return model.Asset{}, ErrAssetNotFound
	}
	return cloneAsset(asset), nil
}

func (m *Manager) ReplaceAsset(assetID string, req model.UpdateAssetRequest) (model.Asset, error) {
	if assetID == "" {
		return model.Asset{}, ErrMissingIdentifier
	}
	if req.AssetID != "" && req.AssetID != assetID {
		return model.Asset{}, fmt.Errorf("%w: path assetId %q does not match body assetId %q", ErrInvalidRequest, assetID, req.AssetID)
	}
	if !isSupportedStatus(req.Status) {
		return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedStatus, req.Status)
	}
	template, err := TemplateFor(req.AssetType)
	if err != nil {
		return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedAsset, req.AssetType)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return model.Asset{}, ErrAssetNotFound
	}

	asset.AssetType = req.AssetType
	asset.Status = req.Status
	asset.Metrics = m.initialMetricsLocked(template)
	for metricName, metric := range req.Metrics {
		if isKnownMetric(template, metricName) {
			asset.Metrics[metricName] = metric
		}
	}
	if req.Status != model.AssetStatusFault {
		asset.ActiveFaults = []model.FaultType{}
	}
	if len(asset.ActiveFaults) > 0 {
		asset.Status = model.AssetStatusFault
		m.applyFaultsLocked(asset)
	}
	asset.UpdatedAt = time.Now().UTC()

	return cloneAsset(asset), nil
}

func (m *Manager) PatchAsset(assetID string, req model.PatchAssetRequest) (model.Asset, error) {
	if assetID == "" {
		return model.Asset{}, ErrMissingIdentifier
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return model.Asset{}, ErrAssetNotFound
	}

	if req.AssetType != nil && *req.AssetType != asset.AssetType {
		template, err := TemplateFor(*req.AssetType)
		if err != nil {
			return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedAsset, *req.AssetType)
		}
		asset.AssetType = *req.AssetType
		asset.Metrics = m.initialMetricsLocked(template)
		asset.ActiveFaults = []model.FaultType{}
	}

	if req.Status != nil {
		if !isSupportedStatus(*req.Status) {
			return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedStatus, *req.Status)
		}
		asset.Status = *req.Status
		if *req.Status != model.AssetStatusFault {
			asset.ActiveFaults = []model.FaultType{}
		}
	}

	template, err := TemplateFor(asset.AssetType)
	if err != nil {
		return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedAsset, asset.AssetType)
	}
	for metricName, metric := range req.Metrics {
		if metric == nil || !isKnownMetric(template, metricName) {
			continue
		}
		asset.Metrics[metricName] = *metric
	}

	if len(asset.ActiveFaults) > 0 {
		asset.Status = model.AssetStatusFault
		m.applyFaultsLocked(asset)
	}
	asset.UpdatedAt = time.Now().UTC()

	return cloneAsset(asset), nil
}

func (m *Manager) DeleteAsset(assetID string) error {
	if assetID == "" {
		return ErrMissingIdentifier
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.assets[assetID]; !ok {
		return ErrAssetNotFound
	}
	delete(m.assets, assetID)
	return nil
}

func (m *Manager) InjectFault(assetID string, faultType model.FaultType) (model.Asset, error) {
	if assetID == "" {
		return model.Asset{}, ErrMissingIdentifier
	}
	if !isSupportedFault(faultType) {
		return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedFault, faultType)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return model.Asset{}, ErrAssetNotFound
	}

	if !hasFault(asset, faultType) {
		asset.ActiveFaults = append(asset.ActiveFaults, faultType)
	}
	asset.Status = model.AssetStatusFault
	m.applyFaultsLocked(asset)
	asset.UpdatedAt = time.Now().UTC()

	return cloneAsset(asset), nil
}

func (m *Manager) ListFaults(assetID string) ([]model.FaultType, error) {
	asset, err := m.GetAsset(assetID)
	if err != nil {
		return nil, err
	}
	return asset.ActiveFaults, nil
}

func (m *Manager) ReplaceFaults(assetID string, faultTypes []model.FaultType) (model.Asset, error) {
	if assetID == "" {
		return model.Asset{}, ErrMissingIdentifier
	}
	for _, faultType := range faultTypes {
		if !isSupportedFault(faultType) {
			return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedFault, faultType)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return model.Asset{}, ErrAssetNotFound
	}

	asset.ActiveFaults = dedupeFaults(faultTypes)
	if len(asset.ActiveFaults) == 0 {
		asset.Status = model.AssetStatusRunning
		m.tickAssetLocked(asset)
	} else {
		asset.Status = model.AssetStatusFault
		m.restoreNormalMetricsLocked(asset)
		m.applyFaultsLocked(asset)
	}
	asset.UpdatedAt = time.Now().UTC()

	return cloneAsset(asset), nil
}

func (m *Manager) PatchFaults(assetID string, addFaults, removeFaults []model.FaultType) (model.Asset, error) {
	if assetID == "" {
		return model.Asset{}, ErrMissingIdentifier
	}
	for _, faultType := range append(addFaults, removeFaults...) {
		if !isSupportedFault(faultType) {
			return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedFault, faultType)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return model.Asset{}, ErrAssetNotFound
	}

	removeSet := make(map[model.FaultType]struct{}, len(removeFaults))
	for _, faultType := range removeFaults {
		removeSet[faultType] = struct{}{}
	}

	next := make([]model.FaultType, 0, len(asset.ActiveFaults)+len(addFaults))
	for _, faultType := range asset.ActiveFaults {
		if _, remove := removeSet[faultType]; !remove {
			next = append(next, faultType)
		}
	}
	next = append(next, addFaults...)
	asset.ActiveFaults = dedupeFaults(next)

	if len(asset.ActiveFaults) == 0 {
		asset.Status = model.AssetStatusRunning
		m.tickAssetLocked(asset)
	} else {
		asset.Status = model.AssetStatusFault
		m.restoreNormalMetricsLocked(asset)
		m.applyFaultsLocked(asset)
	}
	asset.UpdatedAt = time.Now().UTC()

	return cloneAsset(asset), nil
}

func (m *Manager) DeleteFault(assetID string, faultType model.FaultType) (model.Asset, error) {
	if faultType == "" {
		return m.ClearFaults(assetID)
	}
	if !isSupportedFault(faultType) {
		return model.Asset{}, fmt.Errorf("%w: %s", ErrUnsupportedFault, faultType)
	}
	return m.PatchFaults(assetID, nil, []model.FaultType{faultType})
}

func (m *Manager) ClearFaults(assetID string) (model.Asset, error) {
	if assetID == "" {
		return model.Asset{}, ErrMissingIdentifier
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	asset, ok := m.assets[assetID]
	if !ok {
		return model.Asset{}, ErrAssetNotFound
	}
	asset.ActiveFaults = []model.FaultType{}
	asset.Status = model.AssetStatusRunning
	m.tickAssetLocked(asset)
	asset.UpdatedAt = time.Now().UTC()

	return cloneAsset(asset), nil
}

func (m *Manager) runTicker(ctx context.Context) {
	ticker := time.NewTicker(m.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tickAll()
		}
	}
}

func (m *Manager) runPusher(ctx context.Context) {
	ticker := time.NewTicker(m.pushEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.pushClient.Push(ctx, m.ListAssets()); err != nil {
				m.logger.Warn("telemetry push failed", "error", err)
			}
		}
	}
}

func (m *Manager) tickAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, asset := range m.assets {
		m.tickAssetLocked(asset)
		asset.UpdatedAt = time.Now().UTC()
	}
}

func (m *Manager) tickAssetLocked(asset *model.Asset) {
	if asset.Status == model.AssetStatusStopped {
		return
	}
	if len(asset.ActiveFaults) > 0 {
		asset.Status = model.AssetStatusFault
		m.applyFaultsLocked(asset)
		return
	}

	asset.Status = model.AssetStatusRunning
	template, err := TemplateFor(asset.AssetType)
	if err != nil {
		return
	}

	for _, metric := range template.Metrics {
		current, ok := asset.Metrics[metric.Name]
		if !ok {
			current = model.MetricValue{Unit: metric.Unit}
		}

		var next float64
		if metric.Sticky {
			next = current.Value + metric.Drift + m.randomBetween(-0.1, 0.1)
			if next <= metric.Min {
				next = metric.Max
			}
		} else {
			span := metric.Max - metric.Min
			delta := m.randomBetween(-span*0.08, span*0.08)
			next = current.Value + delta
			if next < metric.Min || next > metric.Max {
				next = m.randomBetween(metric.Min, metric.Max)
			}
		}
		asset.Metrics[metric.Name] = model.MetricValue{Value: m.round(next), Unit: metric.Unit}
	}
}

func (m *Manager) initialMetricsLocked(template AssetTemplate) map[string]model.MetricValue {
	metrics := make(map[string]model.MetricValue, len(template.Metrics))
	for _, metric := range template.Metrics {
		value := m.randomBetween(metric.Min, metric.Max)
		if metric.Sticky {
			value = metric.Max
		}
		metrics[metric.Name] = model.MetricValue{Value: m.round(value), Unit: metric.Unit}
	}
	return metrics
}

func (m *Manager) restoreNormalMetricsLocked(asset *model.Asset) {
	template, err := TemplateFor(asset.AssetType)
	if err != nil {
		return
	}

	for _, metric := range template.Metrics {
		current, ok := asset.Metrics[metric.Name]
		if !ok || current.Value < metric.Min || current.Value > metric.Max {
			value := m.randomBetween(metric.Min, metric.Max)
			if metric.Sticky {
				value = metric.Max
			}
			asset.Metrics[metric.Name] = model.MetricValue{Value: m.round(value), Unit: metric.Unit}
			continue
		}
		current.Unit = metric.Unit
		asset.Metrics[metric.Name] = current
	}
}

func (m *Manager) applyFaultsLocked(asset *model.Asset) {
	for _, fault := range asset.ActiveFaults {
		switch fault {
		case model.FaultTypeHighVibration:
			setIfPresent(asset, "vibration_mms", 6.5)
		case model.FaultTypeOverheating:
			setIfPresent(asset, "motor_temp_c", 120)
			setIfPresent(asset, "oil_temp_c", 120)
			setIfPresent(asset, "coolant_temp_c", 120)
			setIfPresent(asset, "engine_temp_c", 120)
		case model.FaultTypeLowPressure:
			setIfPresent(asset, "water_pressure_bar", 1.1)
			setIfPresent(asset, "air_pressure_psi", 45)
			setIfPresent(asset, "flow_rate_lpm", 40)
		case model.FaultTypeFuelLeak:
			setIfPresent(asset, "fuel_level_pct", 5)
			setIfPresent(asset, "fuel_level_liters", 25)
		case model.FaultTypePowerSurge:
			setIfPresent(asset, "voltage_v", 460)
			setIfPresent(asset, "frequency_hz", 58)
			setIfPresent(asset, "motor_rpm", 1850)
			setIfPresent(asset, "engine_rpm", 2800)
		}
	}
}

func isSupportedFault(faultType model.FaultType) bool {
	switch faultType {
	case model.FaultTypeHighVibration,
		model.FaultTypeOverheating,
		model.FaultTypeLowPressure,
		model.FaultTypeFuelLeak,
		model.FaultTypePowerSurge:
		return true
	default:
		return false
	}
}

func isSupportedStatus(status model.AssetStatus) bool {
	switch status {
	case model.AssetStatusRunning, model.AssetStatusStopped, model.AssetStatusFault:
		return true
	default:
		return false
	}
}

func isKnownMetric(template AssetTemplate, metricName string) bool {
	for _, metric := range template.Metrics {
		if metric.Name == metricName {
			return true
		}
	}
	return false
}

func dedupeFaults(faultTypes []model.FaultType) []model.FaultType {
	seen := make(map[model.FaultType]struct{}, len(faultTypes))
	deduped := make([]model.FaultType, 0, len(faultTypes))
	for _, faultType := range faultTypes {
		if _, ok := seen[faultType]; ok {
			continue
		}
		seen[faultType] = struct{}{}
		deduped = append(deduped, faultType)
	}
	return deduped
}

func hasFault(asset *model.Asset, faultType model.FaultType) bool {
	for _, active := range asset.ActiveFaults {
		if active == faultType {
			return true
		}
	}
	return false
}

func setIfPresent(asset *model.Asset, metricName string, value float64) {
	metric, ok := asset.Metrics[metricName]
	if !ok {
		return
	}
	metric.Value = value
	asset.Metrics[metricName] = metric
}

func (m *Manager) randomBetween(minValue, maxValue float64) float64 {
	return minValue + m.rng.Float64()*(maxValue-minValue)
}

func (m *Manager) round(value float64) float64 {
	scale := math.Pow(10, float64(m.roundDigits))
	return math.Round(value*scale) / scale
}

func cloneAsset(asset *model.Asset) model.Asset {
	clone := model.Asset{
		AssetID:      asset.AssetID,
		AssetType:    asset.AssetType,
		Status:       asset.Status,
		Metrics:      make(map[string]model.MetricValue, len(asset.Metrics)),
		ActiveFaults: make([]model.FaultType, 0, len(asset.ActiveFaults)),
		UpdatedAt:    asset.UpdatedAt,
	}
	clone.ActiveFaults = append(clone.ActiveFaults, asset.ActiveFaults...)
	for name, metric := range asset.Metrics {
		clone.Metrics[name] = metric
	}
	return clone
}
