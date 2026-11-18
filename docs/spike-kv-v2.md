# Spike: minimal KV v2 contract (pending item 3)

Ticket 11. Method: **web sources only**. This environment has no `vault`, `terraform`/`tofu` or `hvac` (checked: `which`, `import hvac`). No real request was captured.

## 1. `GET /v1/<mount>/data/<path>`

Success response (200, `Content-Type: application/json`). Fields per the official KV v2 API docs and `api/kv_v2.go`; the bytes below are assembled by us, not captured:

```json
{"request_id":"00000000-0000-0000-0000-000000000000","lease_id":"","renewable":false,"lease_duration":0,"data":{"data":{"password":"s3cret","username":"ann"},"metadata":{"created_time":"2018-03-22T02:24:06.945319214Z","custom_metadata":null,"deletion_time":"","destroyed":false,"version":1}},"wrap_info":null,"warnings":null,"auth":null}
```

- Envelope = the standard Vault `api.Secret`. The envelope fields that the CLI/SDK actually consume are `data` (and `warnings`); `request_id`, `lease_*`, `renewable`, `wrap_info`, `auth` are the standard format and can stay zeroed/null.
- `data.data`: map of strings (Fields, Attributes, `TOTP`).
- `data.metadata`: `version` (int), `created_time`, `deletion_time` (`""` = not deleted), `destroyed` (bool), `custom_metadata` (map or null). The Go SDK (`KVv2.Get`) requires only the `data` key in `secret.Data`; `metadata` is decoded without requiring any field. Even so, emitting the full set (fixed values: `version:1`, `destroyed:false`, `deletion_time:""`) is cheap and avoids surprises in clients that access `metadata.version` (hvac accesses `["data"]["metadata"]["version"]` in some paths).
- Error: 404 with `{"errors":[]}`; 403 with `{"errors":["permission denied"]}`; 405 with `{"errors":[...]}`. The `{"errors":[...]}` error body is the Vault convention. The CLI, on 404, tries to parse the body and treats an empty/dataless body as "No value found".
- The `?version=N` query is sent only if the client asks for a version; the spec says to ignore the query, which is compatible.

## 2. `sys/internal/ui/mounts/<path>` and the `vault` CLI

- `vault kv get` **calls** `GET /v1/sys/internal/ui/mounts/<path>` before reading (`kvPreflightVersionRequest`, `command/kv_helpers.go`, read at tag v1.15.0). Without that response the CLI does not know whether it is v1 or v2 and does not insert `/data/`.
- It reads from the response: `data.path` (string, mount with a trailing `/`, e.g. `"secret/"`) and `data.options.version` (string `"2"`). Missing/null `options`, or a version other than `"2"`, => treated as v1 and reads the literal `GET /v1/<path>` (wrong for our server).
- **404 on the preflight => assumes v1**, not an error. 403 or another error => the CLI aborts with the error. So the server really must answer 200 there.
- Proposed minimal response (derived from the code above, not verified empirically):

```json
{"data":{"path":"secret/","type":"kv","options":{"version":"2"}}}
```

  The full envelope in the standard format is preferable (`request_id`, `lease_id`, ...), since the parse is `api.ParseSecret`; only `data` is read. Careful: the code does an unchecked type assertion (`options.(map)`, `version.(string)`), so `version` must be a **string**, not a number.
- The request carries the token in `X-Vault-Token`. The `<path>` received is the user's argument (e.g. `secret/foo/bar`); the server must answer the same mount `path` for any path inside it. This **widens the spec's surface**: one extra GET on `/v1/sys/internal/ui/mounts/...` (authenticated, 200 with the JSON above). Note: the `secret` mount is only the prefix; our server must pin a mount (e.g. `secret/`).
- Workarounds that skip the preflight: `vault read secret/data/foo` (raw read), and `vault kv get` has no flag to skip it (not found in the docs). For the `run --vault` use case, the typical child (CLI, Terraform, hvac, curl) decides: hvac and curl do not need the endpoint.
- Not verified: whether `kv get -mount=secret foo` also does the preflight (from the `kv_helpers.go` code read, the call happens in `isKVv2`; `kv_get.go` could not be fetched, so this is inference).

## 3. Terraform (`hashicorp/vault` provider)

Source: `internal/provider/meta.go` and the schema in `provider.go` (main branch).

- By default it **creates a child token**: `Auth().Token().Create(...)` (= `POST /v1/auth/token/create`) with `DisplayName` (`token_name`, default `terraform`), `TTL` and `ExplicitMaxTTL` (`max_lease_ttl_seconds`, default 1200), `Renewable=false`. A server that does not implement `auth/token/create` breaks the provider.
- `skip_child_token = true` (provider) or the equivalent env var skips the creation; it uses the configured token directly. **Required configuration: `skip_child_token = true`.** The env var name was not confirmed.
- It also calls `auth/token/lookup-self` at configuration (fails on error or an empty result, and warns if the TTL is low, except for the `root` policy). The server **must answer `GET /v1/auth/token/lookup-self`** with minimal `data`, or the provider does not initialize, even with `skip_child_token`. The exact fields read (`ttl`, `policies`, ...) were not verified.
- `Sys().SealStatus()` (`GET /v1/sys/seal-status`) to detect the version, skippable with `skip_get_vault_version = true` or `vault_version_override`.
- Provisional conclusion: supporting Terraform requires `skip_child_token=true`, `skip_get_vault_version=true` (or the override) and a minimal `auth/token/lookup-self`. Decide whether this is in scope (the spec today says read-only KV v2).

## 4. hvac

Source: `hvac/api/secrets_engines/kv_v2.py`.

- `client.secrets.kv.v2.read_secret_version(path, mount_point="secret")` does `GET /v1/{mount}/data/{path}` and returns the whole JSON (envelope); the user reads `["data"]["data"]`. No `sys/` call in that module. The version goes as a query only if given.
- On 404 it raises `InvalidPath`; for a deleted version, it uses `deletion_time` from `metadata`.
- Not verified: whether `hvac.Client(...)` calls `lookup-self` on construction or in `is_authenticated()` (only if the user calls it; `is_authenticated` uses `lookup-self`). If the user does not call it, there is no extra request.

## 5. Suggested decisions

1. Implement `GET /v1/<mount>/data/<path>` with the JSON from section 1 (fixed metadata).
2. Add `GET /v1/sys/internal/ui/mounts/<path>` (JSON from section 2) if the `vault kv get` CLI is a target; it is cheap.
3. Terraform stays out of scope unless `auth/token/lookup-self` is accepted; document `skip_child_token=true`.
4. Before freezing, run `vault server -dev` or a capture server once with the real tools.

## 6. NOT verified empirically

Everything. No local tool; no request observed. In particular:
- the exact response bytes and the tolerance to missing fields (inferred from the Go/Python code and the docs);
- the CLI preflight with `-mount`; the latest CLI version (reading done at tag v1.15.0);
- the minimal response accepted on `lookup-self` and the `skip_child_token` env var name;
- hvac's behavior on client construction;
- handling of `Authorization: Bearer` by each client (the CLI and the provider use `X-Vault-Token`).
