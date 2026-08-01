package simulator

import (
	"fmt"

	"equipment-telemetry-simulator/internal/model"
)

type AssetTemplate struct {
	Type    model.AssetType
	Metrics []model.MetricDefinition
}

var templates = map[model.AssetType]AssetTemplate{
	model.AssetTypeWaterPump: {
		Type: model.AssetTypeWaterPump,
		Metrics: []model.MetricDefinition{
			{Name: "water_pressure_bar", Unit: "bar", Min: 3.5, Max: 4.5},
			{Name: "flow_rate_lpm", Unit: "lpm", Min: 120, Max: 150},
			{Name: "vibration_mms", Unit: "mm/s", Min: 0.5, Max: 1.2},
			{Name: "motor_temp_c", Unit: "C", Min: 50, Max: 65},
		},
	},
	model.AssetTypeAirCompressor: {
		Type: model.AssetTypeAirCompressor,
		Metrics: []model.MetricDefinition{
			{Name: "air_pressure_psi", Unit: "psi", Min: 90, Max: 110},
			{Name: "oil_temp_c", Unit: "C", Min: 60, Max: 75},
			{Name: "motor_rpm", Unit: "rpm", Min: 1400, Max: 1500},
		},
	},
	model.AssetTypeDieselGenerator: {
		Type: model.AssetTypeDieselGenerator,
		Metrics: []model.MetricDefinition{
			{Name: "voltage_v", Unit: "V", Min: 380, Max: 400},
			{Name: "frequency_hz", Unit: "Hz", Min: 49.5, Max: 50.5},
			{Name: "fuel_level_pct", Unit: "%", Min: 0, Max: 100, Sticky: true, Drift: -0.35},
			{Name: "coolant_temp_c", Unit: "C", Min: 80, Max: 90},
		},
	},
	model.AssetTypeHeavyTruck: {
		Type: model.AssetTypeHeavyTruck,
		Metrics: []model.MetricDefinition{
			{Name: "engine_rpm", Unit: "rpm", Min: 1000, Max: 2200},
			{Name: "speed_kmh", Unit: "km/h", Min: 0, Max: 80},
			{Name: "engine_temp_c", Unit: "C", Min: 85, Max: 95},
			{Name: "fuel_level_liters", Unit: "L", Min: 0, Max: 500, Sticky: true, Drift: -1.2},
		},
	},
}

func TemplateFor(assetType model.AssetType) (AssetTemplate, error) {
	template, ok := templates[assetType]
	if !ok {
		return AssetTemplate{}, fmt.Errorf("unsupported assetType %q", assetType)
	}
	return template, nil
}

func SupportedAssetTypes() []model.AssetType {
	return []model.AssetType{
		model.AssetTypeWaterPump,
		model.AssetTypeAirCompressor,
		model.AssetTypeDieselGenerator,
		model.AssetTypeHeavyTruck,
	}
}
