-- genesismesh: connects Luanti/Mineclonia game events to the GenesisMesh
-- Game Bridge (see bridge/ and section 4.2 of
-- GenesisMesh-Luanti-Server-Requirements.md).
--
-- This mod holds no GenesisMesh private keys. It only talks to the bridge
-- over HTTP and enforces the decisions the bridge returns.

genesismesh = {}

-- ---------------------------------------------------------------------
-- Configuration
-- ---------------------------------------------------------------------

genesismesh.bridge_url = minetest.settings:get("genesismesh.bridge_url") or "http://127.0.0.1:8080"
genesismesh.refresh_interval = tonumber(minetest.settings:get("genesismesh.refresh_interval")) or 5

-- Protected demo area (section 4.5). Configurable via minetest.conf so a
-- server operator can place it without editing Lua.
local function setting_pos(name, default)
	local raw = minetest.settings:get(name)
	if not raw then
		return default
	end
	local x, y, z = raw:match("^%s*(-?%d+)%s*,%s*(-?%d+)%s*,%s*(-?%d+)%s*$")
	if not x then
		return default
	end
	return { x = tonumber(x), y = tonumber(y), z = tonumber(z) }
end

genesismesh.demo_area = {
	name = "demo-area",
	pos1 = setting_pos("genesismesh.demo_area_pos1", { x = -16, y = -16, z = -16 }),
	pos2 = setting_pos("genesismesh.demo_area_pos2", { x = 16, y = 16, z = 16 }),
}

local http = minetest.request_http_api and minetest.request_http_api()
if not http then
	minetest.log("warning",
		"[genesismesh] HTTP API not available; add this mod to secure.http_mods " ..
		"or secure.trusted_mods in minetest.conf. GenesisMesh checks are disabled " ..
		"and every protected action will be denied.")
end

-- player name -> { identity = "...", capabilities = { [cap] = area_or_true, ... } }
local player_state = {}

-- ---------------------------------------------------------------------
-- Bridge HTTP client
-- ---------------------------------------------------------------------

local function bridge_request(method, path, body, callback)
	if not http then
		callback(nil, "http api unavailable")
		return
	end
	http.fetch({
		url = genesismesh.bridge_url .. path,
		method = method,
		timeout = 5,
		data = body and minetest.write_json(body) or nil,
		extra_headers = { "Content-Type: application/json" },
	}, function(res)
		if not res.succeeded or res.code < 200 or res.code >= 300 then
			callback(nil, ("bridge request failed: code=%s"):format(tostring(res.code)))
			return
		end
		local ok, decoded = pcall(minetest.parse_json, res.data)
		if not ok or decoded == nil then
			callback(nil, "bridge returned invalid JSON")
			return
		end
		callback(decoded, nil)
	end)
end

-- Rebuild the local capability cache for a player from the bridge.
local function refresh_capabilities(name)
	bridge_request("GET", "/v1/identities/" .. minetest.urlencode(name) .. "/capabilities", nil,
		function(resp, err)
			if err then
				minetest.log("warning", ("[genesismesh] refresh failed for %s: %s"):format(name, err))
				return
			end
			local caps = {}
			for _, grant in ipairs(resp.capabilities or {}) do
				caps[grant.capability] = grant.area ~= "" and grant.area or true
			end
			player_state[name] = {
				identity = resp.identity and resp.identity.genesismesh_id,
				capabilities = caps,
			}
		end)
end

-- Ask the bridge to make a live boundary decision (used for the protected
-- demo area, where a stale local cache could let a revoked player back in).
function genesismesh.check(name, capability, area, callback)
	bridge_request("POST", "/v1/check", {
		player_name = name,
		capability = capability,
		area = area or "",
	}, function(resp, err)
		if err then
			-- Fail closed: an unreachable bridge must not open a protected
			-- boundary (section 7, "safe and clear result for protected actions").
			callback(false, err)
			return
		end
		callback(resp.allowed, resp.reason)
	end)
end

-- ---------------------------------------------------------------------
-- Player lifecycle
-- ---------------------------------------------------------------------

local refresh_timers = {}

minetest.register_on_joinplayer(function(player)
	local name = player:get_player_name()
	bridge_request("POST", "/v1/identities/link", { player_name = name }, function(resp, err)
		if err then
			minetest.log("warning", ("[genesismesh] identity link failed for %s: %s"):format(name, err))
			return
		end
		player_state[name] = {
			identity = resp.identity and resp.identity.genesismesh_id,
			capabilities = {},
		}
		refresh_capabilities(name)
	end)

	local function tick()
		if not minetest.get_player_by_name(name) then
			return
		end
		refresh_capabilities(name)
		refresh_timers[name] = minetest.after(genesismesh.refresh_interval, tick)
	end
	refresh_timers[name] = minetest.after(genesismesh.refresh_interval, tick)
end)

minetest.register_on_leaveplayer(function(player)
	local name = player:get_player_name()
	player_state[name] = nil
	if refresh_timers[name] then
		refresh_timers[name] = nil
	end
end)

-- ---------------------------------------------------------------------
-- Protected demo area enforcement
-- ---------------------------------------------------------------------

local function in_demo_area(pos)
	local a, b = genesismesh.demo_area.pos1, genesismesh.demo_area.pos2
	return pos.x >= math.min(a.x, b.x) and pos.x <= math.max(a.x, b.x)
		and pos.y >= math.min(a.y, b.y) and pos.y <= math.max(a.y, b.y)
		and pos.z >= math.min(a.z, b.z) and pos.z <= math.max(a.z, b.z)
end

local function deny(name, message)
	minetest.chat_send_player(name, minetest.colorize("#ff5555", "[genesismesh] " .. message))
end

-- world.build / world.destroy inside the demo area go through a live
-- bridge check; everything else stays on normal, fast, local Luanti
-- permissions (section 7).
local old_is_protected = minetest.is_protected
minetest.is_protected = function(pos, name)
	if not in_demo_area(pos) then
		return old_is_protected(pos, name)
	end
	if minetest.check_player_privs(name, { protection_bypass = true }) then
		return false
	end

	-- Fail closed synchronously while the async bridge check runs: if the
	-- player's cached grants don't already show the capability, treat the
	-- action as denied for this call and let the async check confirm/refresh
	-- state for the next attempt.
	local state = player_state[name]
	local allowed = state and (state.capabilities["region.demo.build"] == true
		or state.capabilities["region.demo.build"] == genesismesh.demo_area.name)

	genesismesh.check(name, "region.demo.build", genesismesh.demo_area.name, function(ok, reason)
		if not ok then
			deny(name, "build denied in the protected demo area: " .. tostring(reason))
		end
	end)

	if not allowed then
		return true -- protected: deny
	end
	return old_is_protected(pos, name)
end

-- Entry check: deny movement into the demo area without region.demo.enter.
minetest.register_globalstep(function(dtime)
	for _, player in ipairs(minetest.get_connected_players()) do
		local name = player:get_player_name()
		local exempt = minetest.check_player_privs(name, { protection_bypass = true })
		local pos = player:get_pos()
		if not exempt and in_demo_area(pos) then
			local state = player_state[name]
			local allowed = state and (state.capabilities["region.demo.enter"] == true
				or state.capabilities["region.demo.enter"] == genesismesh.demo_area.name)
			if not allowed then
				deny(name, "entry denied: the protected demo area requires region.demo.enter")
				player:set_pos(genesismesh.demo_area.pos1)
			end
		end
	end
end)

-- ---------------------------------------------------------------------
-- Admin commands for the delegation demo (section 4.5)
-- ---------------------------------------------------------------------

minetest.register_chatcommand("gm_delegate", {
	params = "<player> <capability> [ttl_seconds]",
	description = "Delegate a GenesisMesh capability to a player as this admin's authority.",
	privs = { server = true },
	func = function(caller_name, param)
		local target, capability, ttl = param:match("^(%S+)%s+(%S+)%s*(%d*)$")
		if not target then
			return false, "usage: /gm_delegate <player> <capability> [ttl_seconds]"
		end
		local caller_state = player_state[caller_name]
		if not caller_state or not caller_state.identity then
			return false, "your GenesisMesh identity is not linked yet"
		end
		bridge_request("POST", "/v1/delegate", {
			authority_identity = caller_state.identity,
			player_name = target,
			capability = capability,
			area = genesismesh.demo_area.name,
			ttl_seconds = tonumber(ttl) or 0,
		}, function(resp, err)
			if err then
				minetest.chat_send_player(caller_name, "[genesismesh] delegate failed: " .. err)
				return
			end
			minetest.chat_send_player(caller_name,
				("[genesismesh] granted %s to %s (grant %s)"):format(capability, target, resp.id))
			refresh_capabilities(target)
		end)
		return true, "requesting delegation..."
	end,
})

minetest.register_chatcommand("gm_revoke", {
	params = "<grant_id>",
	description = "Revoke a GenesisMesh capability grant by ID.",
	privs = { server = true },
	func = function(caller_name, param)
		local grant_id = param:match("^(%S+)$")
		if not grant_id then
			return false, "usage: /gm_revoke <grant_id>"
		end
		bridge_request("POST", "/v1/revoke", { grant_id = grant_id, actor = caller_name }, function(resp, err)
			if err then
				minetest.chat_send_player(caller_name, "[genesismesh] revoke failed: " .. err)
				return
			end
			minetest.chat_send_player(caller_name, "[genesismesh] revoked " .. grant_id)
			if resp.identity then
				for name, state in pairs(player_state) do
					if state.identity == resp.identity then
						refresh_capabilities(name)
					end
				end
			end
		end)
		return true, "requesting revocation..."
	end,
})

minetest.log("action", "[genesismesh] mod loaded, bridge=" .. genesismesh.bridge_url)
