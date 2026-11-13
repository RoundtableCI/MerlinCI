# schemas

JSON Schemas for the open agent configuration format (slide 16) and core records
(Phase 0 steps 03 and 04, slide 19).

| Schema | Describes |
|---|---|
| `profile.schema.json` | `Profile`: company and approved product profile, claims, north star, channels, objectives. `Audiences`: audience hypotheses and competitors |
| `role.schema.json` | `RuntimeRole`: one of the runtime roles. `Role`: a logical role with its runtime role, responsibility, model, tools, permissions, evaluation, limits and enabled flag |
| `policy.schema.json` | Autonomy tiers: automatic, conditional, explicit authorization |
| `budget.schema.json` | Spend caps, alert thresholds, exchange rates, cost per paid call, limit behaviour |
| `models.schema.json` | The model allowlist, with the prices used to cap each call |
| `task.schema.json` | A unit of work created by the orchestrator |
| `artifact.schema.json` | A validated output with provenance: sources, run, role, cost |
| `workflow.schema.json` | Workflow template |

Configuration documents share one envelope: `apiVersion: merlinci/v1alpha1`, `kind`,
`metadata.name` and `spec`. Unknown fields are rejected. Money is a decimal string such as
`"150.00"`, never a number. Tasks and artifacts are records, not configuration, and have no
envelope.

The schemas are embedded in the binaries (`schemas.go`). Check a configuration with
`merlinctl validate <path>`.
