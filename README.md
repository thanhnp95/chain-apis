# Chain APIs

A high-performance blockchain data API service for Bitcoin and Litecoin. This service synchronizes blockchain data and provides RESTful APIs for querying blocks, transactions, addresses, and UTXOs.

## Features

- Multi-chain support (Bitcoin and Litecoin)
- Real-time block synchronization with automatic reorg handling
- High-performance storage using Pebble (RocksDB-compatible)
- RESTful API with JSON responses
- Address balance tracking
- UTXO (spent/unspent) tracking

## Configuration

Copy the sample configuration file and update it with your settings:

```bash
cp "config sample.yaml" config.yaml
```

### Configuration Options

| Section | Field | Description |
|---------|-------|-------------|
| `server.port` | API server port | Default: `8089` |
| `server.host` | API server host | Default: `0.0.0.0` |
| `pebble.path` | Base path for Pebble databases | Default: `./data/pebble` |

**Note:** Each chain uses a separate database for better performance. The actual database paths are:
- Bitcoin: `{pebble.path}/btc`
- Litecoin: `{pebble.path}/ltc`
| `bitcoin.enabled` | Enable Bitcoin sync | `true` or `false` |
| `bitcoin.host` | Bitcoin RPC host:port | e.g., `localhost:8334` |
| `bitcoin.user` | Bitcoin RPC username | |
| `bitcoin.pass` | Bitcoin RPC password | |
| `bitcoin.cert` | Path to RPC TLS certificate | Required if TLS enabled |
| `bitcoin.disable_tls` | Disable TLS for RPC | `true` or `false` |
| `bitcoin.http_mode` | Use HTTP instead of WebSocket | `true` or `false` |
| `bitcoin.start_height` | Block height to start syncing from | `0` for genesis |
| `litecoin.*` | Same options as bitcoin | |

## Running the Server

```bash
go run cmd/server/main.go
```

## API Reference

Base URL: `http://localhost:8089/api/v1/{chain}`

Where `{chain}` is either `btc` or `ltc`.

### Health Check

```
GET /health
```

**Response:**
```json
{
  "status": "ok"
}
```

---

### Blocks

#### Get Latest Block

```
GET /api/v1/{chain}/blocks/latest
```

**Response:**
```json
{
  "hash": "000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d",
  "height": 800000,
  "version": 536870912,
  "previous_hash": "00000000000000000002a7c4c1e48d76c5a37902165a270156b7a8d72728a054",
  "merkle_root": "...",
  "timestamp": "2023-07-23T12:00:00Z",
  "bits": "17053894",
  "nonce": 1234567890,
  "tx_count": 2500,
  "size": 1500000,
  "weight": 3993000,
  "chain": "btc"
}
```

#### Get Block by Height

```
GET /api/v1/{chain}/blocks/height/{height}
```

**Parameters:**
- `height` (path): Block height (integer)

#### Get Block by Hash

```
GET /api/v1/{chain}/blocks/{hash}
```

**Parameters:**
- `hash` (path): Block hash (64 character hex string)

#### Get Block Transactions

```
GET /api/v1/{chain}/blocks/{hash}/transactions
```

**Response:**
```json
{
  "block_hash": "000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d",
  "count": 2500,
  "transactions": [...]
}
```

---

### Transactions

#### Get Transaction

```
GET /api/v1/{chain}/transactions/{txid}
```

**Parameters:**
- `txid` (path): Transaction ID (64 character hex string)

**Response:**
```json
{
  "txid": "abc123...",
  "block_hash": "000000...",
  "block_height": 800000,
  "version": 2,
  "lock_time": 0,
  "size": 250,
  "vsize": 166,
  "weight": 661,
  "value": 50000000,
  "fee": 1000,
  "is_coinbase": false,
  "timestamp": "2023-07-23T12:00:00Z",
  "chain": "btc",
  "confirmations": 1000,
  "vins": [...],
  "vouts": [...]
}
```

#### Get Transaction Inputs (Vins)

```
GET /api/v1/{chain}/transactions/{txid}/vins
```

**Response:**
```json
{
  "txid": "abc123...",
  "count": 2,
  "vins": [
    {
      "vin_index": 0,
      "prev_txid": "def456...",
      "prev_vout_index": 1,
      "script_sig": "...",
      "sequence": 4294967295,
      "witness": ["..."],
      "chain": "btc"
    }
  ]
}
```

#### Get Specific Vin

```
GET /api/v1/{chain}/transactions/{txid}/vins/{index}
```

#### Get Transaction Outputs (Vouts)

```
GET /api/v1/{chain}/transactions/{txid}/vouts
```

**Response:**
```json
{
  "txid": "abc123...",
  "count": 2,
  "vouts": [
    {
      "vout_index": 0,
      "value": 50000000,
      "script_pubkey": "76a914...",
      "type": "pubkeyhash",
      "addresses": ["1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"],
      "spent": true,
      "spent_by_txid": "ghi789...",
      "spent_by_vin": 0,
      "chain": "btc"
    }
  ]
}
```

#### Get Specific Vout

```
GET /api/v1/{chain}/transactions/{txid}/vouts/{index}
```

---

### Addresses

#### Get Address Info

```
GET /api/v1/{chain}/addresses/{address}
```

**Response:**
```json
{
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "balance": 6800000000,
  "total_received": 6850000000,
  "total_sent": 50000000,
  "tx_count": 100,
  "chain": "btc"
}
```

**Note:** All values are in satoshis (1 BTC = 100,000,000 satoshis).

#### Get Address Inputs (Spending History)

```
GET /api/v1/{chain}/addresses/{address}/vins
```

**Response:**
```json
{
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "count": 50,
  "vins": [...]
}
```

#### Get Address Outputs (Receiving History)

```
GET /api/v1/{chain}/addresses/{address}/vouts
```

**Response:**
```json
{
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "count": 100,
  "vouts": [...]
}
```

#### Get Address Transactions

```
GET /api/v1/{chain}/addresses/{address}/transactions
```

**Response:**
```json
{
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "count": 100,
  "transactions": [
    {
      "txid": "abc123...",
      "block_hash": "000000...",
      "block_height": 800000,
      "version": 2,
      "lock_time": 0,
      "size": 250,
      "vsize": 166,
      "weight": 661,
      "fee": 1000,
      "value": 50000000,
      "is_coinbase": false,
      "timestamp": "2023-07-23T12:00:00Z",
      "chain": "btc",
      "confirmations": 1000
    }
  ]
}
```

---

## Error Responses

All endpoints return errors in the following format:

```json
{
  "error": "Error message description"
}
```

**HTTP Status Codes:**
- `200 OK` - Success
- `400 Bad Request` - Invalid request parameters
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

---

## Data Models

### Block

| Field | Type | Description |
|-------|------|-------------|
| `hash` | string | Block hash |
| `height` | integer | Block height |
| `version` | integer | Block version |
| `previous_hash` | string | Previous block hash |
| `merkle_root` | string | Merkle root of transactions |
| `timestamp` | string | Block timestamp (ISO 8601) |
| `bits` | string | Difficulty target |
| `nonce` | integer | Nonce value |
| `tx_count` | integer | Number of transactions |
| `size` | integer | Block size in bytes |
| `weight` | integer | Block weight |
| `chain` | string | Chain identifier (btc/ltc) |

### Transaction

| Field | Type | Description |
|-------|------|-------------|
| `txid` | string | Transaction ID |
| `block_hash` | string | Block hash containing this tx |
| `block_height` | integer | Block height |
| `version` | integer | Transaction version |
| `lock_time` | integer | Lock time |
| `size` | integer | Transaction size in bytes |
| `vsize` | integer | Virtual size |
| `weight` | integer | Transaction weight |
| `fee` | integer | Transaction fee (satoshis) |
| `value` | integer | Total output value (satoshis) |
| `is_coinbase` | boolean | Is coinbase transaction |
| `timestamp` | string | Transaction timestamp |
| `chain` | string | Chain identifier |
| `confirmations` | integer | Number of confirmations |

### Vin (Transaction Input)

| Field | Type | Description |
|-------|------|-------------|
| `vin_index` | integer | Input index in transaction |
| `prev_txid` | string | Previous transaction ID |
| `prev_vout_index` | integer | Previous output index |
| `script_sig` | string | Signature script |
| `sequence` | integer | Sequence number |
| `witness` | array | Witness data (SegWit) |
| `chain` | string | Chain identifier |

### Vout (Transaction Output)

| Field | Type | Description |
|-------|------|-------------|
| `vout_index` | integer | Output index in transaction |
| `value` | integer | Output value (satoshis) |
| `script_pubkey` | string | Public key script |
| `type` | string | Output type (pubkeyhash, scripthash, etc.) |
| `addresses` | array | Associated addresses |
| `spent` | boolean | Whether output is spent |
| `spent_by_txid` | string | Transaction that spent this output |
| `spent_by_vin` | integer | Input index that spent this output |
| `chain` | string | Chain identifier |

### Address

| Field | Type | Description |
|-------|------|-------------|
| `address` | string | Address string |
| `balance` | integer | Current balance (satoshis) |
| `total_received` | integer | Total received (satoshis) |
| `total_sent` | integer | Total sent (satoshis) |
| `tx_count` | integer | Number of transactions |
| `chain` | string | Chain identifier |

---

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│   Bitcoin Node  │     │  Litecoin Node  │
│    (btcd)       │     │    (ltcd)       │
└────────┬────────┘     └────────┬────────┘
         │                       │
         │ RPC/WebSocket         │ RPC/WebSocket
         │                       │
         └───────────┬───────────┘
                     │
              ┌──────▼──────┐
              │   Syncer    │
              │ (per chain) │
              └──────┬──────┘
                     │
              ┌──────▼──────┐
              │   Pebble    │
              │  Database   │
              └──────┬──────┘
                     │
              ┌──────▼──────┐
              │  REST API   │
              │   (Gin)     │
              └─────────────┘
```

## License

MIT License
