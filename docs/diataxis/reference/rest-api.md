---
title: "REST API Reference"
description: "Complete reference for the dvr hypervisor daemon REST API — endpoints, request/response schemas, authentication, and error codes."
product: drover-code
audience: [platform-operator, agent]
doc_type: reference
topics:
  - deployment
  - tenancy
surface: repo-docs
---

# REST API Reference

The `dvr` daemon exposes a minimal JSON REST API over HTTP on a configurable port. All endpoints require Bearer token authentication unless the daemon is started without a `--token` value.

**Base URL**: `http://{host}:{port}` (default port: `8080`)

---

## Authentication

Every request must carry an `Authorization` header with a Bearer token matching the value passed to `--token` on daemon start.

```
Authorization: Bearer <token>
```

**Responses when unauthenticated:**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{"error": "unauthorized"}
```

---

## Endpoints

### `POST /v1/instances`

Boot a new Guest VM slice.

#### Request

```http
POST /v1/instances HTTP/1.1
Authorization: Bearer <token>
Content-Type: application/json
```

**Body schema:**

| Field | Type | Required | Description |
|---|---|---|---|
| `image_name` | `string` | ✅ | Path to the base rootfs ext4 image (Firecracker) or project directory (QEMU). |
| `memory_mb` | `integer` | ✅ | RAM to allocate to the Guest VM in megabytes. |
| `env` | `object` | — | Key/value environment variables to inject into the VM. Defaults to `{}`. |

**Example request:**

```json
{
  "image_name": "/var/lib/dvr/images/helloworld.ext4",
  "memory_mb": 128,
  "env": {
    "APP_ENV": "production",
    "LOG_LEVEL": "info"
  }
}
```

#### Response

**`201 Created`** — The instance was successfully booted.

```json
{
  "id": "inst-1",
  "state": "running",
  "ip": "172.20.0.2"
}
```

| Field | Type | Description |
|---|---|---|
| `id` | `string` | Unique identifier for this instance (e.g. `inst-1`, `inst-42`). |
| `state` | `string` | Current state of the instance. Always `"running"` at creation. |
| `ip` | `string` | IP address of the Guest VM on the host bridge network (`172.20.0.0/16`). |

**`400 Bad Request`** — The request body is malformed or missing required fields.

```json
{"error": "invalid json body"}
```

**`500 Internal Server Error`** — The hypervisor could not launch the VM.

```json
{"error": "qemu launch failed: kraft: exit status 1"}
```

---

### `GET /v1/instances/{id}`

Inspect the current state of a running instance.

#### Request

```http
GET /v1/instances/inst-1 HTTP/1.1
Authorization: Bearer <token>
```

#### Response

**`200 OK`**

```json
{
  "id": "inst-1",
  "state": "running",
  "ip": "172.20.0.2"
}
```

**`404 Not Found`**

```json
{"error": "instance not found"}
```

---

### `GET /v1/instances/{id}/logs`

Retrieve the accumulated stdout/stderr output from a running instance's console.

#### Request

```http
GET /v1/instances/inst-1/logs HTTP/1.1
Authorization: Bearer <token>
```

#### Response

**`200 OK`**

```json
{
  "logs": "level=info msg=\"kernel boot complete\"\nlevel=info msg=\"starting application\"\n"
}
```

| Field | Type | Description |
|---|---|---|
| `logs` | `string` | All stdout and stderr output accumulated since VM boot, as a raw string. |

**`404 Not Found`**

```json
{"error": "instance not found"}
```

**`500 Internal Server Error`** — Failed to read logs from the internal buffer.

```json
{"error": "..."}
```

---

### `DELETE /v1/instances/{id}`

Terminate a running instance and trigger the full teardown pipeline:

1. Stop the hypervisor process (SIGTERM)
2. Securely shred the ephemeral overlay disk (zero-overwrite + `fdatasync`)
3. Register a cryptographic `DestructionAudit` record
4. Release the host TAP network interface

This is a **synchronous** operation. The response is not returned until all teardown steps complete.

#### Request

```http
DELETE /v1/instances/inst-1 HTTP/1.1
Authorization: Bearer <token>
```

#### Response

**`204 No Content`** — The instance was successfully terminated and sanitized.

*(No body)*

**`404 Not Found`**

```json
{"error": "instance not found"}
```

**`500 Internal Server Error`** — An error occurred during stop or shredding.

```json
{"error": "failed to stop instance: ..."}
```

---

## Error Response Schema

All error responses share the same schema:

```json
{"error": "<human-readable message>"}
```

---

## HTTP Status Code Summary

| Status | Meaning |
|---|---|
| `200 OK` | Request succeeded (GET) |
| `201 Created` | Instance created and running |
| `204 No Content` | Instance deleted successfully |
| `400 Bad Request` | Malformed request body |
| `401 Unauthorized` | Missing or invalid Bearer token |
| `404 Not Found` | Instance ID does not exist |
| `405 Method Not Allowed` | HTTP verb not supported on this path |
| `500 Internal Server Error` | Hypervisor or teardown failure |

---

## Telemetry

The daemon runs a background telemetry collector that periodically samples running instance state and dispatches metrics to the ClickHouse observability pool. The collection interval is controlled by the `--telemetry` CLI flag (default: 10 seconds). Telemetry does not affect the REST API response time.

---

## Instance ID Format

Instance IDs are monotonically increasing sequences prefixed with `inst-`:

```
inst-1, inst-2, inst-3, ...
```

IDs are assigned at request time and reset when the daemon process restarts. They are **not** persisted across daemon restarts.

---

## See Also

- [CLI Reference](./cli.md)
- [How-to: Deploy the daemon in production](../how-to/deploy-hypervisor-daemon.md)
- [Explanation: Architecture & Security](../explanation/architecture-and-security.md)
