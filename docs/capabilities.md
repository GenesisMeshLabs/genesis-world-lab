# Capability list

From section 4.4 of the requirements. Defined in
`bridge/internal/capability/capability.go`.

| Capability | Meaning | Granted by default? |
| --- | --- | --- |
| `game.connect` | Join the server | Yes |
| `world.read` | Enter and view an area | Yes |
| `world.build` | Place blocks | Yes |
| `world.destroy` | Remove blocks | Yes |
| `chat.send` | Send chat messages | Yes |
| `region.demo.enter` | Enter the protected demo area | No — delegated |
| `region.demo.build` | Build inside the demo area | No — delegated |
| `agent.control` | Give an approved task to an AI agent | No — delegated |
| `server.admin` | Perform limited server administration | No — delegated |

Rules enforced by the bridge (`bridge/internal/capability`):

- A capability grant may be scoped to an area and/or time-limited (`ttl_seconds`).
- An identity can only delegate a capability it currently holds
  (`Store.Delegate` returns `ErrNotHeld` otherwise).
- A grant is revocable at any time and stops applying immediately.
- Every grant, check, and revocation is written to the audit log.
