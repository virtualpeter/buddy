#!/usr/bin/env bash
# Vendor Jamf Platform API OpenAPI specs for go:embed.
#
# Source: Jamf-Concepts/jamf-cli (MIT), specs/platform/*.json. The Platform API
# Gateway does not serve a single live schema like Jamf Pro (/api/schema), so we
# ship a snapshot in the binary and refresh it on demand with this script.
#
# Run `make sync-platform-schema` after Jamf publishes updates, then commit the
# refreshed JSON under internal/buddy/platformschema/specs/.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
dest="$root/internal/buddy/platformschema/specs"
base="https://raw.githubusercontent.com/Jamf-Concepts/jamf-cli/main/specs/platform"

specs=(
	blueprints-api.json
	device-groups-api.json
	device-inventory-api.json
	device-management-actions-api.json
	Declaration-reporting-openapi.json
	jamf-compliance-benchmark-engine-api.json
)

mkdir -p "$dest"
for f in "${specs[@]}"; do
	echo "fetching $f"
	curl -fsSL "$base/$f" -o "$dest/$f"
done

echo "vendored ${#specs[@]} Platform API specs into internal/buddy/platformschema/specs"
