# DCM Policy Manager

A REST API service for managing and evaluating [OPA (Open Policy Agent)](https://www.openpolicyagent.org/) policies within the DCM ecosystem. The service provides full CRUD operations for policy resources and an internal engine API for evaluating service instance requests against those policies.

The API follows [AEP (API Enhancement Proposals)](https://aep.dev/) standards for resource-oriented design and uses [RFC 7807](https://tools.ietf.org/html/rfc7807) Problem Details for error responses.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Building](#building)
  - [Running Locally](#running-locally)
  - [Running with Containers](#running-with-containers)
- [API Reference](#api-reference)
  - [Policy Management API (Port 8080)](#policy-management-api-port-8080)
  - [Policy Evaluation API (Port 8081)](#policy-evaluation-api-port-8081)
- [Writing Policies](#writing-policies)
  - [References](#references)
  - [Rego Policy Structure](#rego-policy-structure)
  - [OPA Input Format](#opa-input-format)
  - [OPA Output Format](#opa-output-format)
  - [Policy Examples](#policy-examples)
  - [Constraints](#constraints)
  - [Service Provider Constraints](#service-provider-constraints)
  - [Label Selectors](#label-selectors)
  - [Evaluation Order and Priority](#evaluation-order-and-priority)
- [Configuration](#configuration)
- [Development Guide](#development-guide)
  - [Project Structure](#project-structure)
  - [Code Generation](#code-generation)
  - [Testing](#testing)
  - [AEP Compliance](#aep-compliance)
  - [CI/CD](#cicd)
- [License](#license)

## Architecture Overview

DCM Policy Manager runs two HTTP servers concurrently:

| Server | Default Port | Purpose |
|--------|-------------|---------|
| **Public API** | 8080 | Policy CRUD operations (external-facing) |
| **Engine API** | 8081 | Policy evaluation (internal, called by other services) |

```
                        ┌──────────────────────────────────┐
  Policy CRUD           │         Policy Manager           │
  (port 8080)  ────────>│                                  │
                        │   ┌────────────┐  ┌───────────┐  │        ┌──────────────┐
                        │   │  Handlers   │──│  Service   │──────────│  PostgreSQL   │
                        │   └────────────┘  └───────────┘  │        └──────────────┘
  Evaluation            │                        │         │
  (port 8081)  ────────>│                   ┌─────────┐    │
                        │                   │OPA Engine│   │
                        │                   │(embedded)│   │
                        │                   └─────────┘    │
                        └──────────────────────────────────┘
```

The service follows a 3-tier architecture: **Handler** (HTTP concerns) -> **Service** (business logic) -> **Store** (data access via GORM). Rego code and policy metadata are both stored in the database. An embedded OPA engine compiles policies from the database on startup and after every CRUD mutation.

## Getting Started

### Prerequisites

- **Go** 1.25.5+
- **PostgreSQL** 16+ (or SQLite for development)
- **Podman** or **Docker** with Compose (for containerized setup)

### Building

```bash
make build          # Build binary to bin/policy-manager
make fmt            # Format code
make vet            # Run go vet
make tidy           # Tidy module dependencies
```

### Running Locally

1. Start PostgreSQL (or use SQLite by setting `DB_TYPE=sqlite`).

2. Set environment variables (see [Configuration](#configuration)):

```bash
export DB_TYPE=pgsql
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=policy-manager
export DB_USER=admin
export DB_PASSWORD=adminpass
```

3. Run the service:

```bash
make run
```

4. Verify:

```bash
curl http://localhost:8080/api/v1alpha1/health
# {"status":"healthy","path":"health"}
```

### Running with Containers

The `compose.yaml` provides a fully configured stack with PostgreSQL and the Policy Manager:

```bash
make e2e-up         # Start all services (builds container image)
make e2e-down       # Stop and remove all services and volumes
```

This starts:
- **PostgreSQL 16** on port 5432
- **Policy Manager** on ports 8080 (public API) and 8081 (engine API)

## API Reference

### Policy Management API (Port 8080)

Base URL: `/api/v1alpha1`

Full OpenAPI specification: [`api/v1alpha1/openapi.yaml`](api/v1alpha1/openapi.yaml)

#### Health Check

```
GET /api/v1alpha1/health
```

#### Create a Policy

```bash
# With server-generated ID
curl -X POST http://localhost:8080/api/v1alpha1/policies \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "Region Enforcement",
    "policy_type": "GLOBAL",
    "priority": 100,
    "enabled": true,
    "label_selector": {"environment": "production"},
    "rego_code": "package policies.region\n\nmain := {\n  \"rejected\": false,\n  \"patch\": {\"region\": \"us-east-1\"},\n  \"selected_provider\": \"aws\"\n}"
  }'
```

Example response (201 Created) for the request above. With server-generated ID, `id` and `path` are assigned by the server; with `?id=region-enforcement`, the response would use that id and `path`: `policies/region-enforcement`.

```json
{
  "path": "policies/a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "display_name": "Region Enforcement",
  "policy_type": "GLOBAL",
  "label_selector": {"environment": "production"},
  "priority": 100,
  "enabled": true,
  "rego_code": "package policies.region\n\nmain := {\n  \"rejected\": false,\n  \"patch\": {\"region\": \"us-east-1\"},\n  \"selected_provider\": \"aws\"\n}",
  "create_time": "2026-01-09T10:30:00Z",
  "update_time": "2026-01-09T10:30:00Z"
}
```

```bash
# With client-specified ID
curl -X POST "http://localhost:8080/api/v1alpha1/policies?id=region-enforcement" \
  -H "Content-Type: application/json" \
  -d '{ ... }'
```

#### Get a Policy

```
GET /api/v1alpha1/policies/{policyId}
```

Example response (200 OK):

```json
{
  "path": "policies/region-enforcement",
  "id": "region-enforcement",
  "display_name": "Region Enforcement",
  "description": "Enforces region constraints for production workloads",
  "policy_type": "GLOBAL",
  "label_selector": {"environment": "production"},
  "priority": 100,
  "enabled": true,
  "rego_code": "package policies.region\n\nmain := {\n  \"rejected\": false,\n  \"patch\": {\"region\": \"us-east-1\"},\n  \"selected_provider\": \"aws\"\n}",
  "create_time": "2026-01-09T10:30:00Z",
  "update_time": "2026-01-09T15:45:00Z"
}
```

#### List Policies

```bash
# Basic listing
GET /api/v1alpha1/policies

# With filtering
GET /api/v1alpha1/policies?filter=policy_type='GLOBAL' AND enabled=true

# With ordering
GET /api/v1alpha1/policies?order_by=priority asc

# With pagination
GET /api/v1alpha1/policies?max_page_size=10&page_token=<token>
```

Supported filter fields: `policy_type` (`GLOBAL`, `USER`), `enabled` (`true`, `false`).

Supported order fields: `priority`, `display_name`, `create_time` (each with `asc` or `desc`).

Note: `Polices`, returned in a `List` call, will have an empty string in their `rego_code` field

#### Update a Policy (Partial)

Uses JSON Merge Patch ([RFC 7396](https://tools.ietf.org/html/rfc7396)). Only provided fields are updated; omitted fields are unchanged.

```bash
curl -X PATCH http://localhost:8080/api/v1alpha1/policies/{policyId} \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "priority": 50,
    "enabled": false
  }'
```

**Immutable fields** (ignored if sent): `path`, `id`, `policy_type`, `create_time`, `update_time`.

#### Delete a Policy

```
DELETE /api/v1alpha1/policies/{policyId}
```

Returns `204 No Content` on success.

#### Policy Resource Fields

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Resource path `policies/{id}` (read-only) |
| `id` | string | Unique identifier, 1-63 chars (read-only) |
| `display_name` | string | Human-readable name (required on create) |
| `description` | string | Optional description (supports markdown) |
| `policy_type` | string | `GLOBAL` or `USER` (required on create, immutable) |
| `label_selector` | object | Key-value pairs for request matching |
| `priority` | integer | 1-1000, lower = higher priority (default: 500) |
| `rego_code` | string | OPA Rego policy code (required on create) |
| `enabled` | boolean | Whether the policy is active (default: true) |
| `create_time` | datetime | Creation timestamp (read-only) |
| `update_time` | datetime | Last update timestamp (read-only) |

#### Error Responses

All errors follow RFC 7807 Problem Details format:

```json
{
  "type": "NOT_FOUND",
  "status": 404,
  "title": "Resource not found",
  "detail": "Policy 'my-policy' does not exist",
  "instance": "9b56f05a-6d85-54bd-d7a7-d0f572ae387a"
}
```

| HTTP Status | Error Type | When |
|-------------|-----------|------|
| 400 | `INVALID_ARGUMENT` | Invalid request parameters |
| 404 | `NOT_FOUND` | Policy not found |
| 409 | `ALREADY_EXISTS` | Policy with same ID exists |
| 422 | `FAILED_PRECONDITION` | Invalid Rego syntax |
| 500 | `INTERNAL` | Unexpected server error |

### Policy Evaluation API (Port 8081)

Base URL: `/api/v1alpha1`

Full OpenAPI specification: [`api/v1alpha1/engine/openapi.yaml`](api/v1alpha1/engine/openapi.yaml)

#### Evaluate a Request

```bash
curl -X POST http://localhost:8081/api/v1alpha1/policies:evaluateRequest \
  -H "Content-Type: application/json" \
  -d '{
    "service_instance": {
      "spec": {
        "region": "us-east-1",
        "instance_type": "t3.medium",
        "metadata": {
          "labels": {
            "environment": "production",
            "team": "backend"
          }
        }
      }
    }
  }'
```

**Response (200 OK):**

```json
{
  "evaluated_service_instance": {
    "spec": {
      "region": "us-east-1",
      "instance_type": "t3.medium",
      "metadata": {
        "labels": {
          "environment": "production",
          "team": "backend"
        }
      }
    }
  },
  "selected_provider": "aws",
  "status": "MODIFIED"
}
```

| Status | Meaning |
|--------|---------|
| `APPROVED` | Request passed through all policies unchanged |
| `MODIFIED` | One or more policies modified the request |

**Error responses:**

| HTTP Status | Meaning |
|-------------|---------|
| 400 | Invalid request format |
| 406 | A policy explicitly rejected the request |
| 409 | A lower-priority policy conflicted with a higher-priority one |
| 500 | Internal error (policy engine failure, database error, etc.) |

## Writing Policies

This section is for policy implementers who write Rego policies evaluated by the Policy Manager.

### References

- [Open Policy Agent](https://www.openpolicyagent.org/docs)
- [Rego Policy Language](https://www.openpolicyagent.org/docs/policy-language)

### Rego Policy Structure

Every policy must:

1. Declare a `package` (used by OPA to identify the policy).
2. Define a `main` rule that returns a decision object ([Output Format](#opa-output-format)).

```rego
package policies.my_policy

main := {
  "rejected": false,
  "patch": { ... },
  "selected_provider": "aws"
}
```

### OPA Input Format

When a policy is evaluated, OPA receives the following input:

```json
{
  "input": {
    "spec": {
      "region": "us-east-1",
      "instance_type": "t3.medium"
    },
    "provider": "aws",
    "constraints": {
      "region": {"const": "us-east-1"}
    },
    "service_provider_constraints": {
      "allow_list": ["aws", "gcp"],
      "patterns": ["^aws"]
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `input.spec` | The current service instance spec (may be modified by earlier policies) |
| `input.provider` | Currently selected provider (empty string if not yet selected) |
| `input.constraints` | Accumulated per-field constraints from higher-priority policies (absent for first policy) |
| `input.service_provider_constraints` | Accumulated service provider constraints (absent for first policy) |

### OPA Output Format

The `main` rule must return an object with the following fields:

```json
{
  "rejected": false,
  "patch": {
    "region": "us-east-1"
  },
  "constraints": {
    "region": {"const": "us-east-1"}
  },
  "service_provider_constraints": {
    "allow_list": ["aws", "gcp"],
    "patterns": ["^(aws|gcp)$"]
  },
  "selected_provider": "aws"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `rejected` | Yes | Set `true` to reject the request |
| `rejection_reason` | No | Reason string (when `rejected` is `true`) |
| `patch` | No | Partial merge into the current spec (RFC 7396). Only include fields to change. |
| `constraints` | No | Per-field JSON Schema constraints to enforce on lower-priority policies |
| `service_provider_constraints` | No | Restrict which service providers can be selected |
| `selected_provider` | No | Select a service provider |

### Policy Examples

#### Approve without changes

```rego
package policies.passthrough

main := {
  "rejected": false
}
```

#### Reject a request

```rego
package policies.security_check

main := result {
  not input.spec.encryption_enabled
  result := {
    "rejected": true,
    "rejection_reason": "Encryption must be enabled for all services"
  }
}

main := result {
  input.spec.encryption_enabled
  result := {
    "rejected": false
  }
}
```

#### Set a value via patch

The `patch` field uses RFC 7396 JSON Merge Patch semantics: only the fields present in the patch are modified; all other fields in the spec are preserved.

```rego
package policies.enforce_region

main := {
  "rejected": false,
  "patch": {
    "region": "us-east-1"
  },
  "selected_provider": "aws"
}
```

#### Set a value and lock it with a constraint

```rego
package policies.lock_region

main := {
  "rejected": false,
  "patch": {
    "region": "us-east-1"
  },
  "constraints": {
    "region": {"const": "us-east-1"}
  }
}
```

#### Set a default with a range constraint

This sets `cpu_count` to 2 but allows lower-priority policies to change it within 1-4:

```rego
package policies.cpu_default

main := {
  "rejected": false,
  "patch": {
    "cpu_count": 2
  },
  "constraints": {
    "cpu_count": {"minimum": 1, "maximum": 4}
  }
}
```

#### Constrain without setting a value

This limits `cpu_count` to 1-8 without setting a default:

```rego
package policies.cpu_guardrails

main := {
  "rejected": false,
  "constraints": {
    "cpu_count": {"minimum": 1, "maximum": 8}
  }
}
```

#### Conditional logic using input

```rego
package policies.env_routing

main := result {
  input.spec.metadata.labels.environment == "production"
  result := {
    "rejected": false,
    "patch": {
      "region": "us-east-1"
    },
    "constraints": {
      "region": {"enum": ["us-east-1", "us-west-2"]}
    },
    "selected_provider": "aws"
  }
}

main := result {
  input.spec.metadata.labels.environment == "staging"
  result := {
    "rejected": false,
    "patch": {
      "region": "eu-west-1"
    },
    "selected_provider": "gcp"
  }
}
```

#### Constraint-aware policy

Lower-priority policies receive accumulated constraints from higher-priority ones in `input.constraints`. A policy can check existing constraints before making decisions:

```rego
package policies.adjust_cpu

import future.keywords.if

main := result if {
  # Check if there's an existing maximum constraint
  max_cpu := input.constraints.cpu_count.maximum
  result := {
    "rejected": false,
    "patch": {
      "cpu_count": min([input.spec.cpu_count, max_cpu])
    }
  }
}

main := result if {
  not input.constraints.cpu_count.maximum
  result := {
    "rejected": false
  }
}
```

### Constraints

Constraints use JSON Schema keywords to restrict what values lower-priority policies can set for each field. Constraints follow a **tightening-only** rule: a lower-priority policy can never loosen a constraint set by a higher-priority one.

Supported constraint keywords:

| Keyword | Description | Tightening Direction |
|---------|-------------|---------------------|
| `const` | Exact fixed value | Must be identical if both set |
| `enum` | Allowed value set | Intersection (must be non-empty) |
| `minimum` | Minimum numeric value | Can only increase |
| `maximum` | Maximum numeric value | Can only decrease |
| `minLength` | Minimum string length | Can only increase |
| `maxLength` | Maximum string length | Can only decrease |
| `pattern` | Regex string pattern | Additional patterns are ANDed |
| `multipleOf` | Numeric multiple | Must be a multiple of existing |

If a lower-priority policy produces a patch value that violates accumulated constraints, the evaluation returns a `409 Conflict` error.

If a lower-priority policy attempts to loosen a constraint (e.g., increase a `maximum`), the evaluation also returns a `409 Conflict` error.

### Service Provider Constraints

Policies can restrict which service providers are allowed:

```rego
main := {
  "rejected": false,
  "service_provider_constraints": {
    "allow_list": ["aws", "gcp"],
    "patterns": ["^(aws|gcp)$"]
  },
  "selected_provider": "aws"
}
```

- **`allow_list`**: Explicit list of allowed providers. When multiple policies set allow lists, they are intersected (only providers in all lists remain).
- **`patterns`**: Regex patterns that the provider name must match. Patterns from all policies are ANDed.

If a lower-priority policy selects a provider not in the accumulated allow list or not matching all patterns, evaluation returns a `409 Conflict`.

### Label Selectors

Label selectors control which requests a policy applies to. A policy is evaluated only if **all** labels in its selector match the request context.

Matching is performed against two sources:

- **Request labels**: Extracted from the service instance spec at `spec.metadata.labels`.
- **Service type**: The `ServiceType` field in the spec is also available for matching.

| Policy Selector | Request Context | Result |
|----------------|----------------|--------|
| `{}` (empty) | Any | Matches (wildcard) |
| `{env: prod}` | `{env: prod, team: backend}` | Matches |
| `{env: prod, team: backend}` | `{env: prod}` | No match (missing `team`) |
| `{env: prod}` | `{env: staging}` | No match (value mismatch) |

### Evaluation Order and Priority

Policies are evaluated sequentially in the following order:

1. **Policy type**: `GLOBAL` policies first, then `USER` policies.
2. **Priority**: Within each type, lower priority number = evaluated first (1 is highest priority). Priority is unique within a policy type, so no two policies of the same type share the same priority.

Each policy receives the current state of the spec (potentially modified by earlier policies) and accumulated constraints. This means:

- A priority-100 GLOBAL policy runs before a priority-200 GLOBAL policy.
- A GLOBAL policy always runs before a USER policy, regardless of priority.
- Higher-priority policies can set constraints that restrict what lower-priority policies can do.

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `BIND_ADDRESS` | `0.0.0.0:8080` | Public API server listen address |
| `ENGINE_BIND_ADDRESS` | `0.0.0.0:8081` | Engine API server listen address |
| `LOG_LEVEL` | `info` | Logging level |
| `DB_TYPE` | `pgsql` | Database type: `pgsql` or `sqlite` |
| `DB_HOST` | `localhost` | PostgreSQL hostname |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `policy-manager` | Database name |
| `DB_USER` | `admin` | Database user |
| `DB_PASSWORD` | `adminpass` | Database password |

## Development Guide

### Project Structure

```
dcm-policy-manager/
├── api/v1alpha1/                    # OpenAPI specs and generated types
│   ├── openapi.yaml                 # Policy Management API spec (source of truth)
│   ├── types.gen.go                 # Generated Go types
│   ├── spec.gen.go                  # Generated embedded spec
│   └── engine/                      # Policy Evaluation API spec
│       ├── openapi.yaml
│       ├── types.gen.go
│       └── spec.gen.go
├── cmd/policy-manager/
│   └── main.go                      # Application entry point
├── internal/
│   ├── api/
│   │   ├── server/                  # Generated Chi server stubs (public API)
│   │   └── engine/                  # Generated Chi server stubs (engine API)
│   ├── apiserver/                   # Public API HTTP server wrapper
│   ├── engineserver/                # Engine API HTTP server wrapper
│   ├── config/                      # Environment variable configuration
│   ├── handlers/
│   │   ├── v1alpha1/                # Public API request handlers
│   │   └── engine/                  # Engine API request handlers
│   ├── opa/                         # Embedded OPA policy engine
│   ├── service/                     # Business logic layer
│   │   ├── policy.go                # Policy CRUD operations
│   │   ├── evaluation.go            # Policy evaluation logic
│   │   ├── constraints.go           # JSON Schema constraint enforcement
│   │   ├── labelmatcher.go          # Label selector matching
│   │   ├── filter.go                # List filter parsing
│   │   └── orderby.go               # Order-by parsing
│   └── store/                       # Database access layer (GORM)
│       ├── model/                   # Database models
│       ├── policy.go                # Policy data operations
│       └── db.go                    # Database initialization
├── pkg/
│   ├── client/                      # Generated API client (public)
│   └── engineclient/                # Generated API client (engine)
├── test/e2e/                        # End-to-end tests
├── Containerfile                    # Multi-stage container build
├── compose.yaml                     # Docker/Podman Compose for local dev
├── Makefile                         # Build targets
└── tools.go                         # Build tool dependencies
```

### Code Generation

The project uses [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) to generate Go types, server stubs, and client code from the OpenAPI specifications. **After modifying any `openapi.yaml` file, you must regenerate the code:**

```bash
make generate-api           # Regenerate all API code (both public and engine)

# Or generate specific components:
make generate-types         # Public API types
make generate-spec          # Public API embedded spec
make generate-server        # Public API server stubs
make generate-client        # Public API client
make generate-engine-api    # All engine API code

# Verify generated files are in sync:
make check-generate-api
```

CI will fail if generated files are out of sync with the OpenAPI specs.

### Testing

The project uses [Ginkgo](https://onsi.github.io/ginkgo/) as the test framework with [Gomega](https://onsi.github.io/gomega/) matchers.

#### Unit Tests

```bash
make test                              # Run all unit tests
go test -run TestName ./path/to/pkg    # Run a specific test
```

#### End-to-End Tests

E2E tests use the `e2e` build tag and require the full stack (PostgreSQL, Policy Manager) running via Compose:

```bash
# Full cycle (start services, run tests, stop services)
make test-e2e-full

# Or step-by-step:
make e2e-up           # Start services
make test-e2e         # Run E2E tests
make e2e-down         # Stop and clean up

# Run a specific E2E test
go test -v -tags=e2e ./test/e2e -ginkgo.focus="should create policy"
```

E2E tests read the following environment variables:
- `API_URL` (default: `http://localhost:8080/api/v1alpha1`)
- `ENGINE_API_URL` (default: `http://localhost:8081/api/v1alpha1`)

**IDE setup**: For IntelliSense in E2E test files, configure gopls with `-tags=e2e`. The repo includes `.vscode/settings.json` with this configuration. For other editors, add the equivalent setting and reload.

### AEP Compliance

The API specifications are validated against [AEP standards](https://aep.dev/) using [Spectral](https://stoplight.io/spectral):

```bash
make check-aep          # Check all API specs
make check-aep-api      # Check public API only
make check-aep-engine   # Check engine API only
```

The linter configuration is in `.spectral.yaml`.

### CI/CD

GitHub Actions workflows:

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yaml` | All PRs to main | Build and test |
| `e2e.yaml` | PRs to main (non-docs changes) | End-to-end tests |
| `check-generate.yaml` | API file changes | Verify generated code is in sync |
| `check-aep.yaml` | OpenAPI spec changes | AEP standards compliance |
| `build-push-quay.yaml` | Releases | Build and push container image |

### Container Image

The `Containerfile` uses a multi-stage build:

1. **Builder**: Red Hat UBI 9 Go toolset, compiles a static binary.
2. **Runtime**: Red Hat UBI 9 minimal, runs as non-root user (UID 1001), exposes port 8080.

```bash
podman build -f Containerfile -t policy-manager .
```

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
