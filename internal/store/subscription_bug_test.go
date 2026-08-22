package store

import "testing"

func TestBug09_SubscriptionCancelAfterStoreCloseIsSafe(t *testing.T) {
	s := NewMemory()
	_, cancel := s.Subscribe()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	cancel()
}
