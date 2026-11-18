# 11: Spike: KV v2 response format (pending item 3)

**What to build:** Find out the minimal contract that the `vault` CLI, the Terraform provider and `hvac` require to read a KV v2 secret from a local server. Points to settle: the exact JSON of the `GET /v1/<mount>/data/<path>` response (envelope, `data.data`, `data.metadata`); whether the CLI calls `/v1/sys/internal/ui/mounts/<path>` before reading and what it needs to answer there; whether Terraform creates a child token (`auth/token/create`) or accepts `skip_child_token`. The method is to observe the real requests against a capture server or against `vault server -dev`. The result is a short document in `docs/` with the requests, the minimal responses and what stays out of scope. If a tool is missing from the environment, record what was not verified.

**Blocked by:** None (can start immediately)

**Status:** done

- [ ] The KV v2 response format is documented with a byte example
- [ ] It is recorded whether the `vault` CLI requires the `sys/internal/ui/mounts` endpoint and what the minimal response is
- [ ] Terraform's behavior regarding the child token and the required configuration is recorded
- [ ] `hvac`'s behavior is recorded
- [ ] What could not be verified is listed explicitly
