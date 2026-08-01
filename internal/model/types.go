package model

import "time"

type AssetType string

const (
	AssetTypeWaterPump       AssetType = "WATER_PUMP"
	AssetTypeAirCompressor   AssetType = "AIR_COMPRESSOR"
	AssetTypeDieselGenerator AssetType = "DIESEL_GENERATOR"
	AssetTypeHeavyTruck      AssetType = "HEAVY_TRUCK"
)

type AssetStatus string

const (
	AssetStatusRunning AssetStatus = "RUNNING"
	AssetStatusStopped AssetStatus = "STOPPED"
	AssetStatusFault   AssetStatus = "FAULT"
)

type FaultType string

const (
	FaultTypeHighVibration FaultType = "HIGH_VIBRATION"
	FaultTypeOverheating   FaultType = "OVERHEATING"
	FaultTypeLowPressure   FaultType = "LOW_PRESSURE"
	FaultTypeFuelLeak      FaultType = "FUEL_LEAK"
	FaultTypePowerSurge    FaultType = "POWER_SURGE"
)

type MetricDefinition struct {
	Name   string  `json:"name"`
	Unit   string  `json:"unit"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Sticky bool    `json:"sticky,omitempty"`
	Drift  float64 `json:"drift,omitempty"`
}

type MetricValue struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type Asset struct {
	AssetID      string                 `json:"assetId"`
	AssetType    AssetType              `json:"assetType"`
	Status       AssetStatus            `json:"status"`
	Metrics      map[string]MetricValue `json:"metrics"`
	ActiveFaults []FaultType            `json:"activeFaults"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

type RegisterAssetRequest struct {
	AssetID   string    `json:"assetId"`
	AssetType AssetType `json:"assetType"`
}

type UpdateAssetRequest struct {
	AssetID   string                 `json:"assetId,omitempty"`
	AssetType AssetType              `json:"assetType"`
	Status    AssetStatus            `json:"status"`
	Metrics   map[string]MetricValue `json:"metrics,omitempty"`
}

type PatchAssetRequest struct {
	AssetType *AssetType              `json:"assetType,omitempty"`
	Status    *AssetStatus            `json:"status,omitempty"`
	Metrics   map[string]*MetricValue `json:"metrics,omitempty"`
}

type InjectFaultRequest struct {
	AssetID   string    `json:"assetId"`
	FaultType FaultType `json:"faultType"`
}

type ReplaceFaultsRequest struct {
	AssetID    string      `json:"assetId,omitempty"`
	FaultTypes []FaultType `json:"faultTypes"`
}

type PatchFaultsRequest struct {
	AssetID string      `json:"assetId,omitempty"`
	Add     []FaultType `json:"add,omitempty"`
	Remove  []FaultType `json:"remove,omitempty"`
}

type ClearFaultRequest struct {
	AssetID string `json:"assetId"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
