# GenesisMesh Luanti Server Requirements

**Status:** Initial requirements  
**Purpose:** Define why the project matters, what must be built, which tools to use, and where to find them.  
**Technical depth:** High-level only. Detailed design and code will be handled separately.

## 1. Project summary

Build a playable Minecraft-style world using **Luanti** and **Mineclonia**, with **GenesisMesh** added to the server as its trust and authority layer.

The game should show, in a simple and visible way, how humans, AI agents, servers, and organizations can:

- prove who they are;
- receive limited rights;
- delegate some of those rights;
- lose those rights through expiry or revocation;
- work across independent authorities without one central owner.

The first version is a **GenesisMesh demonstration and test environment**, not a new commercial game.

## 2. Why build it

GenesisMesh concepts can be difficult to explain through APIs, documents, or JSON alone. A game world makes the result visible.

For example, a player tries to enter or build in a protected area and is refused. An authority then delegates the required right. The same player tries again and succeeds. When the right is revoked, access stops.

This project is promising because it provides:

- **A visual GenesisMesh demo:** people can see trust, delegation, and revocation happen.
- **A safe protocol laboratory:** new trust rules can be tested without using a critical production system.
- **A strong AI-agent use case:** an AI character can receive only the exact rights needed for its work.
- **A cross-sovereign example:** two server operators can cooperate while each keeps control of its own identities, rules, and world areas.
- **An open stack:** the game engine, game content, server mod, and GenesisMesh integration can all be inspected and changed.
- **A useful public story:** the project explains GenesisMesh through something familiar and interactive.

## 3. Main goals

The first release must:

1. Start as a normal, playable Mineclonia server.
2. Connect the server to GenesisMesh.
3. Give each test player a GenesisMesh-linked identity.
4. Control important actions through named capabilities.
5. Support temporary delegation.
6. Support immediate revocation.
7. Record clear evidence of important trust decisions.
8. Include one short demo that another person can run and understand.

## 4. What must be built

### 4.1 Playable game server

Use Luanti as the open-source voxel engine and Mineclonia as the ready-made Minecraft-style game.

The server must support:

- one persistent world;
- multiple players;
- normal Mineclonia gameplay;
- server-side mods;
- a protected demonstration area.

### 4.2 GenesisMesh game mod

Create a small server-side Luanti mod called `genesismesh`.

Its purpose is to connect game events to the GenesisMesh Game Bridge. It should:

- notice when a player joins;
- request the player's approved game rights;
- allow or deny selected important actions;
- show a clear message when an action is refused;
- refresh rights when they expire or are revoked.

The mod should not contain GenesisMesh authority private keys.

### 4.3 GenesisMesh Game Bridge

Create a small service between Luanti and GenesisMesh.

Its purpose is to translate simple game requests into GenesisMesh identity, capability, delegation, boundary, revocation, and audit operations.

The bridge should:

- hold the server's GenesisMesh credentials safely;
- link a game player to a GenesisMesh identity;
- return the capabilities that a player or agent may use;
- check important boundary decisions;
- detect expiry and revocation;
- create a useful audit record.

The bridge is an adapter for the game. It is not a new GenesisMesh protocol.

### 4.4 Capability model

Start with a small set of rights that are easy to demonstrate:

| Capability | Meaning |
| --- | --- |
| `game.connect` | Join the server |
| `world.read` | Enter and view an area |
| `world.build` | Place blocks |
| `world.destroy` | Remove blocks |
| `chat.send` | Send chat messages |
| `region.demo.enter` | Enter the protected demo area |
| `region.demo.build` | Build inside the demo area |
| `agent.control` | Give an approved task to an AI agent |
| `server.admin` | Perform limited server administration |

Rights must be limited by identity, action, area, and time when appropriate. A person or agent must not be able to delegate a right they do not already hold.

### 4.5 Demonstration scenario

The first demo should contain two independent authorities and one protected area.

The demo flow must show:

1. A player joins with a valid identity.
2. The player tries to enter or build in the protected area and is denied.
3. Another authority delegates the required capability for a short time.
4. The player can now enter or build.
5. The capability is revoked or expires.
6. The same action is denied again.
7. The audit record shows who granted the right, what was allowed, and when it ended.

### 4.6 Basic project documentation

The repository must include:

- a short project overview;
- setup instructions;
- how to start and stop the server;
- how to run the demo;
- the initial capability list;
- where logs and audit records can be viewed;
- licensing and reuse information for each main component.

## 5. Tools and where to find them

| Tool | Use in this project | Where to find it |
| --- | --- | --- |
| **Luanti** | Open-source voxel engine, multiplayer server, and mod platform | [Luanti website and download](https://www.luanti.org/en/) · [Luanti source code](https://github.com/luanti-org/luanti) |
| **Mineclonia** | Ready-made Minecraft-style survival game running on Luanti | [Mineclonia on ContentDB](https://content.luanti.org/packages/ryvnf/mineclonia/) · [Mineclonia source code](https://codeberg.org/mineclonia/mineclonia) |
| **Lua** | Language used for the Luanti server mod | [Luanti creator documentation](https://docs.luanti.org/for-creators/) · [Luanti HTTP API](https://docs.luanti.org/for-creators/api/http-api/) |
| **GenesisMesh** | Identity, agreements, capabilities, delegation, boundaries, revocation, and audit | [GenesisMesh website](https://genesismesh.org/) · [GenesisMesh GitHub organization](https://github.com/GenesisMeshLabs) |
| **Go** | Recommended language for the first Game Bridge because it can produce a small service | [Go download](https://go.dev/dl/) |
| **Docker Desktop** | Run the game server, bridge, GenesisMesh services, and supporting services together | [Docker Desktop](https://www.docker.com/products/docker-desktop/) |
| **Git** | Track changes and work safely with forks | [Git for Windows](https://git-scm.com/downloads/win) |
| **Visual Studio Code** | Main editor for Lua, Go, configuration, and Markdown | [Visual Studio Code](https://code.visualstudio.com/) |
| **GitHub** | Host the new project and manage issues and releases | [GenesisMeshLabs on GitHub](https://github.com/GenesisMeshLabs) |
| **Ollama** | Optional later phase for local AI villagers or agents | [Ollama download](https://ollama.com/download) |

## 6. Recommended machine

Use the **personal ASUS with 32 GB RAM and the NVIDIA GPU** as the main development and server machine.

Reasons:

- it is a personal open-source project;
- it avoids mixing GenesisMesh keys and experiments with an employer-managed device;
- the NVIDIA GPU can support local AI agents later;
- 32 GB RAM is enough for the first local server, bridge, and supporting services.

The H&M Mac should remain for work unless H&M explicitly approves personal development on it.

## 7. Important safety requirements

- GenesisMesh private keys must stay in the Game Bridge or an approved secret store.
- Private keys and passwords must never be committed to Git.
- The Luanti mod must receive decisions, not unrestricted authority credentials.
- Important grants, denials, delegations, expiries, and revocations must be auditable.
- Normal block activity should remain local and fast. GenesisMesh checks should focus on identity, authority changes, protected boundaries, delegation, and revocation.
- The world and the minimum configuration needed to restore it must be backed up.
- The first server should be private or limited to invited testers.

## 8. Delivery phases

### Phase 1: Playable base

- Run Luanti with Mineclonia on the ASUS.
- Create a persistent multiplayer world.
- Add a protected demo area.

### Phase 2: GenesisMesh connection

- Create the `genesismesh` Luanti mod.
- Create the GenesisMesh Game Bridge.
- Link test players to GenesisMesh identities.
- Apply the first capability checks.

### Phase 3: Delegation demo

- Add two independent authorities.
- Demonstrate denial, delegation, approval, expiry, and revocation.
- Make the related audit evidence easy to view.

### Phase 4: Public-quality demo

- Add a one-command or similarly simple startup process.
- Write the demo guide.
- Record a short demonstration video.
- Publish the project under the GenesisMeshLabs organization.

### Later phases

- AI villagers with GenesisMesh identities and limited capabilities;
- trust between multiple Luanti servers;
- organization- or country-themed authorities;
- a simple visual trust and audit dashboard;
- controlled economy, trading, or land ownership;
- public test server.

## 9. Minimum acceptance criteria

The first useful release is complete when:

- a new tester can start the project using the written guide;
- two players can join and play;
- a player without the required capability is denied access to the demo area;
- an authorized identity can grant a limited, temporary capability;
- the player is allowed after the grant;
- access stops after revocation or expiry;
- the full change is visible in the audit record;
- no GenesisMesh private key is stored in the Lua mod or source repository;
- normal gameplay remains responsive when GenesisMesh is available;
- failure of the bridge produces a safe and clear result for protected actions.

## 10. Out of scope for the first release

The first release will not include:

- changes to the Luanti C++ engine;
- a large public server;
- a full in-game economy;
- custom graphics or a complete new game;
- Kubernetes or cloud production hosting;
- an advanced web dashboard;
- many AI agents;
- checks against GenesisMesh for every block action;
- replacement of all normal Luanti permissions.

## 11. Project principle

**Keep gameplay fast and local. Use GenesisMesh where trust crosses an important boundary.**

This keeps the game enjoyable while making identity, authority, delegation, and revocation visible and verifiable.
