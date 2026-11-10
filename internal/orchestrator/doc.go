// Package orchestrator creates, orders and schedules tasks.
//
// Agents never spawn work themselves: they request delegation and the
// orchestrator decides (deck slide 10, Delegation). It drives the
// signal-to-next-cycle flow (deck slide 9), including the CMO's accept, reject
// or reprioritize step.
package orchestrator
