# Capabilities

Application claims use profile worldlab/v1 inside signed GenesisMesh
MembershipAttestations; no new wire protocol is introduced.

| Right | Enforcement |
| --- | --- |
| game.connect | Approved signed identity link |
| world.read | Movement outside lobby |
| world.build / world.destroy | Both required for normal editing |
| chat.send | Chat hook |
| region.demo.enter | Temporary court entry |
| region.demo.build | Court editing, alongside entry/baseline rights |
| agent.control / server.admin | Reserved, not issued by demo endpoint |

The generic protection hook requires both baseline edit rights and preserves
existing protection. protection_bypass cannot override GenesisMesh.
Court bounds: x20..36, y4..20, z-8..8. Denial returns players to the outside lobby.

Only configured operators with valid signed roots can delegate. TTL 1..3600 seconds
cannot exceed the parent. Authority, capability and area must match; parent expiry
or revocation denies descendants. No unsigned system/world-owner bypass.

Decisions check signatures, identity, scope, expiry, revocation and fresh recognition.
Leases last at most 1.5 seconds, shortened by evidence expiry, measured from request start.
Obsolete async replies are discarded. Upstream freshness is at most 6 seconds, so the
conservative outage bound is 7.5 seconds plus the 0.1-second engine interval. This is bounded
eventual revocation, not instantaneous global denial.
