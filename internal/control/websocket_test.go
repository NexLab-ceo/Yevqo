package control

import (
	"testing"

	"veltrix/internal/shared"
)

func TestHubReplacesOldDeviceConnection(t *testing.T) {
	hub := NewHub()
	oldConn := &AgentConn{Device:&shared.Device{ID:7}}
	newConn := &AgentConn{Device:&shared.Device{ID:7}}
	hub.Add(oldConn)
	hub.Add(newConn)
	got, ok := hub.Get(7)
	if !ok || got != newConn { t.Fatal("hub did not retain the newest device connection") }
	if hub.RemoveConn(oldConn) { t.Fatal("stale connection removed the active connection") }
	got, ok = hub.Get(7)
	if !ok || got != newConn { t.Fatal("stale connection teardown removed the replacement") }
	if !hub.RemoveConn(newConn) { t.Fatal("active connection could not be removed") }
}

func TestAgentConnShutdownFailsPendingCommands(t *testing.T) {
	resultCh := make(chan *shared.Result, 1)
	conn := &AgentConn{done:make(chan struct{}),pending:map[string]chan *shared.Result{"cmd-1":resultCh}}
	conn.shutdown()
	select {
	case <-conn.done:
	default:
		t.Fatal("shutdown did not close done channel")
	}
	select {
	case result := <-resultCh:
		if result.Ok || result.Message != "Agent disconnected" { t.Fatalf("unexpected pending result: %#v",result) }
	default:
		t.Fatal("shutdown did not release pending command")
	}
	if len(conn.pending)!=0 { t.Fatalf("pending commands remain: %d",len(conn.pending)) }
}
