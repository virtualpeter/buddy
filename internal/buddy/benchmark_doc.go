package buddy

import "buddy/internal/docspec"

func init() {
	docspec.Register(BenchmarksTopic)
}

// BenchmarksTopic is the structured doc source for docs/benchmarks.
// Edit here, then run `make gendocs`.
var BenchmarksTopic = docspec.Topic{
	ID:      `docs/benchmarks`,
	Title:   `Compliance benchmarks`,
	Section: `Platform`,
	Keywords: []string{
		`benchmark`,
		`benchmarks`,
		`compliance`,
		`baseline`,
		`baselines`,
	},
	Summary: `Platform compliance benchmarks (beta) — baselines, catalog rules, reports. Requires platform-capable tenant + auth.`,
	Surfaces: []docspec.SurfaceRow{
		{Action: "List / load / mutate", BuddyScript: "`benchmark*` builtins", CLI: "`buddy benchmark …`", Notes: "Baselines, catalog rules, reports"},
	},
	Notes: []string{
		"Platform compliance benchmarks (beta). Requires platform-capable tenant + auth.",
	},
	AgentNotes: []string{
		"Do not invent benchmark HTTP from schema alone when builtins exist.",
		"Do not confuse with Pro smart-group “compliance” criteria or MDM commands.",
	},
	Builtins: []docspec.Builtin{
			{
				Name: `benchmarks`,
				Summary: `List compliance benchmarks`,
				Returns: `list`,
				Params: []docspec.Param{
					{Name: `filter`, Type: `string`, Required: false, Description: `filter`},
				},
			},
			{
				Name: `benchmark`,
				Summary: `Load one benchmark`,
				Returns: `dict`,
				Params: []docspec.Param{
					{Name: `id`, Type: `string`, Required: true, Description: `id`},
				},
			},
		},
	ExamplesBuddy: []string{
		`def main():
    return benchmarks()`,
	},
	Related: []docspec.RelatedLink{
		{Title: `API endpoints`, Ref: `/docs/docs/api-endpoints`},
		{Title: `MCP`, Ref: `/docs/docs/mcp`},
	},
	HelpExport: `benchmarks`,
	BuiltinsHelp: `COMPLIANCE BENCHMARK (beta Platform API)
  benchmarks(tenant?, surface?, auth?)
      Does: List tenant compliance benchmarks.
      → value: JSON (benchmark list)
  benchmark(id=, tenant?, surface?, auth?)
      Does: Fetch one compliance benchmark.
      → value: dict (benchmark record)
      Params: id=: str (benchmark ID).
  benchmark_create(body, tenant?, surface?, auth?)
      Does: Create a compliance benchmark from a JSON body.
      → value: dict (created benchmark)
      Params: body: dict (create JSON).
  benchmark_delete(benchmark_id, tenant?, surface?, auth?)
      Does: Delete a compliance benchmark.
      → None
      Params: benchmark_id: str.
  benchmark_baselines(tenant?, surface?, auth?)
      Does: List mSCP baselines available to the tenant.
      → value: JSON (baseline list)
  benchmark_catalog_rules(baseline?, tenant?, surface?, auth?)
      Does: List catalog rules, optionally for one baseline.
      → value: JSON (rule catalog)
      Params: baseline?: str (baseline ID).
  benchmark_compliance_percentage(benchmark_id, tenant?, surface?, auth?)
      Does: Return overall compliance percentage for a benchmark.
      → value: JSON (percentage report)
      Params: benchmark_id: str.
  benchmark_report_devices(benchmark_id, tenant?, surface?, auth?)
      Does: Report devices covered by a benchmark.
      → value: JSON (device report)
      Params: benchmark_id: str.
  benchmark_report_rules(benchmark_id, tenant?, surface?, auth?)
      Does: Report per-rule stats for a benchmark.
      → value: JSON (rules report)
      Params: benchmark_id: str.

`,
	CLIHelp: `buddy benchmark - Jamf Platform compliance benchmarks (beta)

Usage:
  buddy [global-flags] benchmark <subcommand> [args...]

Subcommands:
  list                                   List tenant benchmarks (JSON)
  get <benchmark-id>                     Fetch one benchmark
  create [--file=<path>|-]               Create from JSON body
  delete <benchmark-id>                  Delete a benchmark
  baselines                              List mSCP baselines
  rules [--baseline=<id>]                Catalog rules for a baseline
  compliance-percentage <benchmark-id>   Overall compliance percentage
  report-devices <benchmark-id>          Devices for a benchmark report
  report-rules <benchmark-id>            Rules stats for a benchmark

Notes:
  Beta Platform API. Requires Platform gateway tenant+auth and
  read|create|delete:pro:compliance-benchmarks as applicable.

Examples:
  buddy benchmark list
  buddy benchmark baselines
  buddy benchmark get <benchmark-id>
`,
}
