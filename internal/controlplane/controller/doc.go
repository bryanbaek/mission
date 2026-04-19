// Package controller orchestrates workflows. Normalizes input, enforces
// business rules, delegates I/O to repositories and gateways. Controllers
// are deterministic and unit-testable; they never touch HTTP or SQL directly.
package controller
