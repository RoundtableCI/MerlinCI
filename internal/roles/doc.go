// Package roles loads role definitions. A role is a configuration and a
// responsibility contract run by one reusable runtime (deck slide 8).
//
// 18 logical roles map onto 8 runtime roles: CMO, Intelligence, Strategist,
// Content, Creative, Distribution, Growth, Analyst. Brand Guardian begins as an
// independent evaluation stage (internal/eval/brandguardian). A role is split
// out only when its tools, workload or evaluation justify it, and is enabled
// only when ready, never merely because its prompt has been written
// (deck slides 8, 13).
package roles
