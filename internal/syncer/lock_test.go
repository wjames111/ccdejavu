package syncer_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/wjames111/ccdejavu/internal/syncer"
)

func TestLockIsExclusive(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "data", "lock")
	unlock, err := syncer.Lock(path)
	if err != nil {
		t.Fatal(err)
	}

	got := make(chan func(), 1)
	go func() {
		second, err := syncer.Lock(path)
		if err != nil {
			t.Error(err)
			return
		}
		got <- second
	}()

	select {
	case <-got:
		t.Fatal("second Lock succeeded while the first was held")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	select {
	case second := <-got:
		second()
	case <-time.After(2 * time.Second):
		t.Fatal("second Lock never got the lock after unlock")
	}
}
