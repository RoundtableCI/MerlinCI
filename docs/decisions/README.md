# Decisions

"Decide these explicitly rather than assume them silently." (slide 18)

Each decision gets its own record (`NNNN-title.md`, from `template.md`) when it is made.

| # | Decision | Status | Affects |
|---|---|---|---|
| 01 | Open-source license | Open — must be chosen before the first public release | `LICENSE` |
| 02 | Organization and repository naming | Decided: `RoundtableCI/MerlinCI` | repo |
| 03 | Temporal or another durable engine | Open — scaffold targets Temporal, the engine named in the deck | `internal/workflow`, `deploy` |
| 04 | Hosting provider and topology | Open | `deploy` |
| 05 | RoundtableCI activation event | Open | `internal/analytics`, `configs/examples/roundtableci` |
| 06 | Competitor list and audience segments | Open | `configs/examples/roundtableci` |
| 07 | Search and research API providers | Open | `internal/integrations/research` |
| 08 | First publishing connector and permissions | Open | `internal/integrations/publishing`, policy |
| 09 | Image and video generation providers | Open | `internal/capabilities/contentstudio` |
| 10 | Email provider and consent model | Open | `internal/integrations/publishing`, policy |
| 11 | Model allowlist and budget limits | Open — deck sets the budget defaults (slide 15); allowlist open | `internal/gateway`, `internal/budget` |
| 12 | Which publishing actions run autonomously | Open | `internal/policy` |
| 13 | Dashboard in v1, or CLI and API first | Open | `web`, `cmd/merlinctl` |
