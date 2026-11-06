package main

import "context"

// The upstream command/cache interface drains shared interim counters before
// saving absolute snapshots. Serialize complete commands, not only SQL writes.
// One service-wide gate fits the deliberately small single-process deployment.
func (s *SQLiteStore) acquireCommand(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.commandGate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *SQLiteStore) releaseCommand() { <-s.commandGate }
