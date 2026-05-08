// Package store provides SQLite-backed persistence for orca using modernc.org/sqlite
// (pure Go, CGO_ENABLED=0). It exposes Open to create a Store with an up-to-date
// schema, and CRUD operations for all core domain types (pods, runs, logs, events).
package store
