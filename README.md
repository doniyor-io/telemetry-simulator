# Generic Equipment Telemetry & Digital Twin Simulator

Lightweight Go microservice for simulating equipment telemetry for Enterprise TOiR, MRO, EAM, and automated maintenance workflows.


## Features

- Thread-safe digital twin state engine
- Normal metric fluctuation every configurable tick interval
- Fault injection for anomaly testing
- Optional webhook push mode for telemetry batches
- Standard library HTTP router

## Run Locally

```bash
go run ./cmd/server
```

The API listens on `http://localhost:8080`.

## Run With Docker Compose

```bash
docker compose up --build
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | HTTP port |
| `TICK_INTERVAL` | `2s` | Asset simulation update interval |
| `PUSH_MODE` | `false` | Enable outbound telemetry push |
| `TARGET_TOIR_URL` | empty | Target URL for telemetry batch HTTP POST |
| `PUSH_INTERVAL` | `TICK_INTERVAL` | Outbound push interval |

## API Examples

### List Assets

```bash
curl -s http://localhost:8080/api/v1/assets | jq
```

### Get Asset

```bash
curl -s http://localhost:8080/api/v1/assets/PUMP-101 | jq
```

### Get Telemetry

```bash
curl -s "http://localhost:8080/api/v1/telemetry?assetId=PUMP-101" | jq
```

or:

```bash
curl -s http://localhost:8080/api/v1/telemetry/PUMP-101 | jq
```

### Register New Asset

```bash
curl -s -X POST http://localhost:8080/api/v1/assets \
  -H "Content-Type: application/json" \
  -d '{"assetId":"COMP-302","assetType":"AIR_COMPRESSOR"}' | jq
```

### Replace Asset

```bash
curl -s -X PUT http://localhost:8080/api/v1/assets/COMP-302 \
  -H "Content-Type: application/json" \
  -d '{"assetType":"AIR_COMPRESSOR","status":"RUNNING"}' | jq
```

### Patch Asset

```bash
curl -s -X PATCH http://localhost:8080/api/v1/assets/COMP-302 \
  -H "Content-Type: application/json" \
  -d '{"status":"STOPPED"}' | jq
```

### Delete Asset

```bash
curl -i -X DELETE http://localhost:8080/api/v1/assets/COMP-302
```

Supported asset types:

- `WATER_PUMP`
- `AIR_COMPRESSOR`
- `DIESEL_GENERATOR`
- `HEAVY_TRUCK`

### Inject Fault

```bash
curl -s -X POST http://localhost:8080/api/v1/faults \
  -H "Content-Type: application/json" \
  -d '{"assetId":"PUMP-101","faultType":"HIGH_VIBRATION"}' | jq
```

The legacy compatibility endpoint also remains available:

```bash
curl -s -X POST http://localhost:8080/api/v1/faults/inject \
  -H "Content-Type: application/json" \
  -d '{"assetId":"PUMP-101","faultType":"HIGH_VIBRATION"}' | jq
```

### List Faults

```bash
curl -s "http://localhost:8080/api/v1/faults?assetId=PUMP-101" | jq
```

### Replace Faults

```bash
curl -s -X PUT http://localhost:8080/api/v1/faults/PUMP-101 \
  -H "Content-Type: application/json" \
  -d '{"faultTypes":["HIGH_VIBRATION","OVERHEATING"]}' | jq
```

### Patch Faults

```bash
curl -s -X PATCH http://localhost:8080/api/v1/faults/PUMP-101 \
  -H "Content-Type: application/json" \
  -d '{"add":["LOW_PRESSURE"],"remove":["HIGH_VIBRATION"]}' | jq
```

### Delete One Fault

```bash
curl -s -X DELETE http://localhost:8080/api/v1/faults/PUMP-101/LOW_PRESSURE | jq
```

Supported fault types:

- `HIGH_VIBRATION`
- `OVERHEATING`
- `LOW_PRESSURE`
- `FUEL_LEAK`
- `POWER_SURGE`

### Clear Faults

```bash
curl -s -X DELETE http://localhost:8080/api/v1/faults/PUMP-101 | jq
```

The legacy compatibility endpoint also remains available:

```bash
curl -s -X POST http://localhost:8080/api/v1/faults/clear \
  -H "Content-Type: application/json" \
  -d '{"assetId":"PUMP-101"}' | jq
```

## Webhook Push Mode

Enable push mode to periodically POST telemetry batches to a TOiR endpoint.

```bash
PUSH_MODE=true TARGET_TOIR_URL=http://localhost:9000/telemetry go run ./cmd/server
```

Payload shape:

```json
{
  "sentAt": "2026-07-31T12:00:00Z",
  "assets": [
    {
      "assetId": "PUMP-101",
      "assetType": "WATER_PUMP",
      "status": "RUNNING",
      "metrics": {
        "water_pressure_bar": { "value": 4.12, "unit": "bar" }
      },
      "activeFaults": [],
      "updatedAt": "2026-07-31T12:00:00Z"
    }
  ]
}
```
