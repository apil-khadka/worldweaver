# WorldWeaver Configuration

## Server Configuration

All server parameters are defined in `internal/config/config.go` and `internal/config/defaults.go`.

### ServerConfig

| Field | Type | Default | Unit | Description |
|-------|------|---------|------|-------------|
| Addr | string | `:8080` | — | HTTP listen address |
| WorldWidth | int | 1024 | cells | World width |
| WorldHeight | int | 512 | cells | World height |
| WorldSeed | int64 | 20260823 | — | Generation seed |
| ChunkSize | int | 64 | cells/edge | Chunk side length |
| TickRate | int | 60 | Hz | Simulation TPS |
| NetworkRate | int | 20 | Hz | Broadcast frequency |
| SnapshotDir | string | `.` | path | Snapshot save directory |
| SnapshotInterval | Duration | 5m | — | Auto-save interval |
| MaxPlayers | int | 16 | — | Maximum concurrent clients |

### Tuning (Gameplay)

| Field | Default | Unit | Description |
|-------|---------|------|-------------|
| InfluenceMax | 100 | points | Max influence pool |
| InfluenceRegenPerSec | 8.0 | points/sec | Regeneration rate |
| RainCostPerSec | 2.0 | points/sec | Rain power drain |
| HeatCostPerSec | 3.0 | points/sec | Heat power drain |
| WindCostPerSec | 1.0 | points/sec | Wind power drain |
| GrowthCostPerSec | 4.0 | points/sec | Growth power drain |
| FireLifetimeTicks | 120 | ticks | Fire burns for ~2 sec |
| EvapTempThreshold | 1000 | 0.1°C | Water→vapor above 100°C |
| MaxInfluenceRadius | 64 | cells | Server-enforced max |
| MaxIntensity | 1.0 | ratio | Server-enforced max |

### Feature Flags

| Flag | Default | Description |
|------|---------|-------------|
| SimPlants | true | Enable plant growth |
| SimFire | true | Enable fire spread |
| SimVapor | true | Enable vapor/steam |
| SimWind | true | Enable wind effects |
| NetCompress | false | Enable RLE compression |
| DebugMetrics | true | Enable /api/metrics |

## CLI Flags

```bash
go run ./cmd/server \
  -addr :8080 \
  -width 1024 \
  -height 512 \
  -seed 20260823 \
  -snapdir ./snapshots
```

## Validation

Configuration is validated at startup. Invalid values produce clear error messages:
- World dimensions must be > 0
- ChunkSize must be 1–WorldWidth
- TickRate must be 1–240 Hz
- NetworkRate must be ≤ TickRate
- SnapshotInterval must be ≥ 10s

## Client Configuration

See `web/src/config/client-config.ts` for frontend configuration.

| Category | Key Values |
|----------|------------|
| Network | reconnectDelay: 1s, maxReconnect: 10s, ping: 3s |
| Renderer | preference: [webgpu, webgl2, canvas2d], scale: 1.0 |
| Input | panSpeed: 8, powerRadius: 24, zoom: 0.25–8.0 |
| Features | webgpu: on, canvas: on, waterAnim: on, fireAnim: on |

## Deployment Configuration

### Docker Environment Variables

#### Backend (`Dockerfile.backend`)

| Variable | Default | Description |
|----------|---------|-------------|
| TZ | UTC | Timezone for logging |

CLI flags are passed via `CMD` in the Dockerfile:
```bash
CMD ["-addr", ":8080", "-snapdir", "/app/data"]
```

#### Frontend (`Dockerfile.frontend`)

| Variable | Default | Description |
|----------|---------|-------------|
| BACKEND_URL | `http://worldweaver-backend:8080` | Backend service URL for nginx proxy |

### Dokploy Setup

| Service | Dockerfile | Docker Context | Port | Domain |
|---------|------------|----------------|------|--------|
| Backend | `Dockerfile.backend` | `.` | 8080 | worldweaverapi.apilkhadka.com.np |
| Frontend | `Dockerfile.frontend` | `.` | 80 | worldweaver.apilkhadka.com.np |

**Branch:** `main`
**Auto-deploy:** Webhook triggered by GitHub Actions on push to main.

### API Endpoints (Backend)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | API info (name, version, status, endpoints) |
| `/health` | GET | Health check (Docker/Dokploy readiness) |
| `/ws` | GET | WebSocket upgrade for game clients |
| `/api/metrics` | GET | Runtime metrics JSON |
