// Package workflow holds the durable workflows and activities (deck slide 12:
// Temporal or equivalent; open decision 03).
//
// Runs resume from the last durable checkpoint and publishing actions are never
// repeated (deck slide 10, Recovery). Every run is inspectable, costed and
// recoverable (deck slide 14).
package workflow
