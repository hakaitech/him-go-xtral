# Himitsu Central Server

This is the central server for Project Himitsu, a "dumb" broker designed to be simple, fast, and stateless.

## Components

The central server consists of three services orchestrated with Docker Compose:

1. **Go API** - Identity management service (hash ↔ pub_key mapping)
2. **Redis** - Identity database
3. **Mosquitto** - MQTT broker for messaging

## Quick Start

### Prerequisites

- Docker
- Docker Compose

### Build and Run

```bash
cd himitsu-server
docker compose build
docker compose up -d
```

### Stop Services

```bash
docker compose down
```

## Services

### API Service (Port 8080)

The API provides three endpoints:

#### 1. Register a Hash
```bash
POST /register
Content-Type: application/json

{
  "hash": "1234567890",
  "pub_key": "-----BEGIN PUBLIC KEY-----\n..."
}
```

Response:
- `201 Created`: Successfully registered
- `409 Conflict`: Hash already taken
- `400 Bad Request`: Invalid input

#### 2. Get Public Key
```bash
GET /get_pub_key?hash=1234567890
```

Response:
- `200 OK`: Returns hash and public key
- `404 Not Found`: Hash not found
- `400 Bad Request`: Invalid hash format

#### 3. Dispose Hash
```bash
POST /dispose
Content-Type: application/json

{
  "hash": "1234567890",
  "signature": "hex_encoded_signature"
}
```

Response:
- `200 OK`: Hash disposed
- `401 Unauthorized`: Invalid signature
- `404 Not Found`: Hash not found

### Redis Service (Port 6379)

Redis stores the hash-to-public-key mappings. The API service connects to it internally.

### Mosquitto MQTT Broker (Port 1883)

MQTT broker for message passing. Configured with:
- Anonymous access allowed
- ACL file for topic restrictions
- Persistence enabled

## Testing

Test the API endpoints:

```bash
# Register a hash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"hash":"test123456","pub_key":"test-public-key"}'

# Get public key
curl "http://localhost:8080/get_pub_key?hash=test123456"
```

## Architecture

```
┌─────────────────┐
│   Go API        │  Port 8080
│  (Identity Mgmt)│
└────────┬────────┘
         │
         ├─────────┐
         │         │
┌────────▼──────┐  │
│    Redis      │  │
│  (Database)   │  │
└───────────────┘  │
                   │
         ┌─────────▼──────┐
         │   Mosquitto    │  Port 1883
         │  (MQTT Broker) │
         └────────────────┘
```

## Development

The vendor directory is used for dependencies to avoid network issues during Docker builds.

To update dependencies:
```bash
cd api
go mod tidy
go mod vendor
```

## Configuration

### Environment Variables

- `GIN_MODE`: Gin framework mode (default: `release`)
- `REDIS_ADDR`: Redis address (default: `redis:6379`)

### Volumes

- `redis_data`: Redis persistent data
- `mosquitto_data`: Mosquitto persistent data
- `mosquitto_log`: Mosquitto logs
