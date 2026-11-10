# configs

Everything company- or role-specific is configuration, not code (deck slides 7, 8).
Nothing RoundtableCI-specific belongs in Go packages.

| Directory | Contents | Deck |
|---|---|---|
| `roles/runtime/` | The 8 runtime roles: CMO, Intelligence, Strategist, Content, Creative, Distribution, Growth, Analyst | slide 8 |
| `roles/logical/` | Logical roles (18 in the plan), each bound to a runtime role, with tools, permissions and evaluation contract. Market Researcher ships first | slides 8, 13, 19 |
| `workflows/` | Workflow templates, starting with the first-release end-to-end workflow | slides 9, 14 |
| `examples/roundtableci/` | Customer one: approved product profile, audience hypotheses, channels, policy and budget | slides 7, 11, 15 |

Every file here validates against a schema in `/schemas`. A role stays disabled until its tools,
permissions and evaluation are ready, never merely because its prompt has been written (slide 13).
