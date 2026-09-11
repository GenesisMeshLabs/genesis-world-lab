# Bridge API

Origin http://127.0.0.1:8789. All routes except health/readiness require
Authorization: Bearer <scoped-token>. JSON is limited to 16 KiB; unknown fields
and trailing JSON are rejected.

| Method/path | Purpose |
| --- | --- |
| GET /healthz | Liveness, mode and world |
| GET /readyz | Trust readiness; 503 unavailable |
| POST /v1/identities/link | Game credential: approved player_name |
| GET /v1/identities/{player}/capabilities | Identity, rights, lease_ms |
| POST /v1/check | player_name, capability, area; audited decision |
| POST /v1/delegate | player_name, capability, area, ttl_seconds; optional parent_id |
| POST /v1/revoke | grant_id; durable local deny then upstream publication |
| GET /v1/audit?after=0 | Up to 200 ordered events after ID |
| GET /v1/status | Identity summaries, grants and trust |

Game delegate/revoke requests additionally need caller_name, a configured
operator authenticated by Luanti. Authority tokens derive actor from scope.
One authority cannot revoke another's grant.

Example grant:
```json
{"player_name":"alice","capability":"region.demo.enter","area":"demo-area","ttl_seconds":60}
```

Save the returned ID. Revoke publication failure returns 503 but preserves local
denial; retry the same revoke to publish it.
Errors: 400 input, 401 bearer, 403 scope/identity, 404 missing grant, 409 capacity,
429 quota, 503 trust/upstream/storage. Loopback bind only; remote upstreams require
HTTPS. Browser cross-origin access is disabled.
