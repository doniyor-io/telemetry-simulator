package model

import "testing"

func TestMetricsMapRoundTrip(t *testing.T) {
	metrics := MetricsMap{
		"temperature_c": {Value: 72.5, Unit: "C"},
		"pressure_bar":  {Value: 4.2, Unit: "bar"},
	}

	value, err := metrics.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}

	var scanned MetricsMap
	if err := scanned.Scan(value); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if scanned["temperature_c"].Value != 72.5 {
		t.Fatalf("temperature_c = %v, want 72.5", scanned["temperature_c"].Value)
	}
}

func TestValidateMetricDefinitionsRejectsDuplicateMetric(t *testing.T) {
	err := ValidateMetricDefinitions([]MetricDefinition{
		{Name: "pressure", Unit: "bar", Min: 1, Max: 2},
		{Name: "pressure", Unit: "bar", Min: 3, Max: 4},
	})
	if err == nil {
		t.Fatal("expected duplicate metric validation error")
	}
}
