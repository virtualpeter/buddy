#!/usr/bin/env bash
# Vendor Jamf Platform API OpenAPI specs for go:embed.
#
# Source: Jamf-Concepts/jamfplatform-go-sdk (MIT), api/*.json — jamf-cli's
# Makefile treats that directory as the normalised source of truth;
# specs/platform/ in jamf-cli is a copy. Official docs and the Jamf MCP
# document the same six native services, but the gateway still has no live
# schema like Jamf Pro (/api/schema): authenticated GETs of /schema and
# /openapi.json are 404; per-service /openapi.json is 403 BAD_PERMISSIONS
# (unknown path), and the portal does not host a downloadable OAS. We ship
# a snapshot. Revisit if Jamf publishes a live schema URL (see AGENTS.md,
# "Platform schema vendoring").
#
# Run `make sync-platform-schema` after Jamf publishes updates, then commit the
# refreshed JSON under internal/buddy/platformschema/specs/.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
dest="$root/internal/buddy/platformschema/specs"
base="https://raw.githubusercontent.com/Jamf-Concepts/jamfplatform-go-sdk/main/api"

# Native Platform services only — do not pull pro_api.json, Classic, Account,
# AI, Audit, or Security Cloud into jamf_schema(api="platform").
specs=(
	blueprints_api.json
	device_group_inventory_api.json
	device_inventory_api.json
	device_management_action_api.json
	declaration_reporting_service.json
	compliance_benchmark_engine.json
)

mkdir -p "$dest"
for f in "${specs[@]}"; do
	echo "fetching $f"
	curl -fsSL "$base/$f" -o "$dest/$f"
done

# Drop previous jamf-cli filenames so go:embed does not keep stale copies.
for stale in \
	blueprints-api.json \
	device-groups-api.json \
	device-inventory-api.json \
	device-management-actions-api.json \
	Declaration-reporting-openapi.json \
	jamf-compliance-benchmark-engine-api.json
do
	rm -f "$dest/$stale"
done

echo "vendored ${#specs[@]} Platform API specs into internal/buddy/platformschema/specs"
