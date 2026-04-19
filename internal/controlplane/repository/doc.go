// Package repository encapsulates Postgres access for the control plane.
// One repository per aggregate (tenants, tenant_users, tenant_tokens,
// semantic_layers, scheduled_reports, ...). Repositories return typed
// structs from internal/controlplane/model and never return pgx types upward.
package repository
