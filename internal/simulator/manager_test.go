package simulator

import (
	"errors"
	"testing"

	"equipment-telemetry-simulator/internal/model"
)

func TestRegisterAssetInitializesMetricsAndEmptyFaults(t *testing.T) {
	manager := NewManager(Config{})

	asset, err := manager.RegisterAsset("PUMP-101", model.AssetTypeWaterPump)
	if err != nil {
		t.Fatalf("register asset: %v", err)
	}

	if asset.Status != model.AssetStatusRunning {
		t.Fatalf("status = %s, want %s", asset.Status, model.AssetStatusRunning)
	}
	if len(asset.ActiveFaults) != 0 {
		t.Fatalf("active faults = %v, want empty", asset.ActiveFaults)
	}
	if asset.ActiveFaults == nil {
		t.Fatal("active faults should encode as an empty array, not null")
	}
	if _, ok := asset.Metrics["water_pressure_bar"]; !ok {
		t.Fatal("missing water_pressure_bar metric")
	}
}

func TestInjectFaultForcesMetricBreach(t *testing.T) {
	manager := NewManager(Config{})
	if _, err := manager.RegisterAsset("PUMP-101", model.AssetTypeWaterPump); err != nil {
		t.Fatalf("register asset: %v", err)
	}

	asset, err := manager.InjectFault("PUMP-101", model.FaultTypeHighVibration)
	if err != nil {
		t.Fatalf("inject fault: %v", err)
	}

	if asset.Status != model.AssetStatusFault {
		t.Fatalf("status = %s, want %s", asset.Status, model.AssetStatusFault)
	}
	if got := asset.Metrics["vibration_mms"].Value; got != 6.5 {
		t.Fatalf("vibration_mms = %v, want 6.5", got)
	}
}

func TestClearFaultsRestoresRunningState(t *testing.T) {
	manager := NewManager(Config{})
	if _, err := manager.RegisterAsset("GEN-401", model.AssetTypeDieselGenerator); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if _, err := manager.InjectFault("GEN-401", model.FaultTypePowerSurge); err != nil {
		t.Fatalf("inject fault: %v", err)
	}

	asset, err := manager.ClearFaults("GEN-401")
	if err != nil {
		t.Fatalf("clear faults: %v", err)
	}

	if asset.Status != model.AssetStatusRunning {
		t.Fatalf("status = %s, want %s", asset.Status, model.AssetStatusRunning)
	}
	if len(asset.ActiveFaults) != 0 {
		t.Fatalf("active faults = %v, want empty", asset.ActiveFaults)
	}
}

func TestPatchAssetCanStopAsset(t *testing.T) {
	manager := NewManager(Config{})
	if _, err := manager.RegisterAsset("TRUCK-501", model.AssetTypeHeavyTruck); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	status := model.AssetStatusStopped

	asset, err := manager.PatchAsset("TRUCK-501", model.PatchAssetRequest{Status: &status})
	if err != nil {
		t.Fatalf("patch asset: %v", err)
	}

	if asset.Status != model.AssetStatusStopped {
		t.Fatalf("status = %s, want %s", asset.Status, model.AssetStatusStopped)
	}
}

func TestReplaceFaultsDeduplicatesAndAppliesFaultState(t *testing.T) {
	manager := NewManager(Config{})
	if _, err := manager.RegisterAsset("GEN-401", model.AssetTypeDieselGenerator); err != nil {
		t.Fatalf("register asset: %v", err)
	}

	asset, err := manager.ReplaceFaults("GEN-401", []model.FaultType{
		model.FaultTypePowerSurge,
		model.FaultTypePowerSurge,
	})
	if err != nil {
		t.Fatalf("replace faults: %v", err)
	}

	if len(asset.ActiveFaults) != 1 {
		t.Fatalf("active faults = %v, want one deduplicated fault", asset.ActiveFaults)
	}
	if got := asset.Metrics["voltage_v"].Value; got != 460 {
		t.Fatalf("voltage_v = %v, want 460", got)
	}
}

func TestRemovingOneFaultRestoresThatFaultMetric(t *testing.T) {
	manager := NewManager(Config{})
	if _, err := manager.RegisterAsset("GEN-401", model.AssetTypeDieselGenerator); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if _, err := manager.ReplaceFaults("GEN-401", []model.FaultType{
		model.FaultTypePowerSurge,
		model.FaultTypeOverheating,
	}); err != nil {
		t.Fatalf("replace faults: %v", err)
	}

	asset, err := manager.DeleteFault("GEN-401", model.FaultTypePowerSurge)
	if err != nil {
		t.Fatalf("delete fault: %v", err)
	}

	if got := asset.Metrics["voltage_v"].Value; got < 380 || got > 400 {
		t.Fatalf("voltage_v = %v, want restored normal range", got)
	}
	if got := asset.Metrics["coolant_temp_c"].Value; got != 120 {
		t.Fatalf("coolant_temp_c = %v, want remaining overheating breach", got)
	}
}

func TestDeleteAssetRemovesAsset(t *testing.T) {
	manager := NewManager(Config{})
	if _, err := manager.RegisterAsset("PUMP-101", model.AssetTypeWaterPump); err != nil {
		t.Fatalf("register asset: %v", err)
	}

	if err := manager.DeleteAsset("PUMP-101"); err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	if _, err := manager.GetAsset("PUMP-101"); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("get deleted asset error = %v, want ErrAssetNotFound", err)
	}
}

func TestRejectsUnsupportedAssetType(t *testing.T) {
	manager := NewManager(Config{})

	_, err := manager.RegisterAsset("X-1", model.AssetType("UNKNOWN"))
	if !errors.Is(err, ErrUnsupportedAsset) {
		t.Fatalf("error = %v, want ErrUnsupportedAsset", err)
	}
}
