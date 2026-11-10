// Package budget caps spend before it is spent (deck slide 15).
//
// RoundtableCI defaults: inference alert and cap of ₹150 per day, inference
// ceiling of ₹3,000 per month, research API ceiling of ₹500 per month. Alerts at
// 70% and 90%; hard stop or degraded service at the limit. Agents can never
// raise their own limit. No autonomous ad spend.
package budget
