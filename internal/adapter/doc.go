// Package adapter defines the hexagonal port for coding agent adapters.
// It exposes the Adapter interface, all related types, and a Registry that
// maps adapter names to Factory functions. Concrete implementations
// (claude-code, codex, aider, opencode) live in separate packages that
// import this one — never the other way around.
package adapter
