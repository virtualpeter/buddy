package buddy

import (
	"bytes"
	"runtime"
	"strconv"
	"sync"
)

// ConfirmAskFunc handles a confirm prompt outside of stdin (e.g. IDE UI).
// It must return whether the user chose yes.
type ConfirmAskFunc func(opts ConfirmOpts) bool

var confirmAskers sync.Map // goroutine id (uint64) → ConfirmAskFunc

// PushConfirmAsk registers fn for the current goroutine so Confirm uses it
// instead of stdin. Restore with the returned function (usually defer).
// Used by buddy serve IDE sessions.
func PushConfirmAsk(fn ConfirmAskFunc) (restore func()) {
	if fn == nil {
		return func() {}
	}
	id := goroutineID()
	prev, _ := confirmAskers.Load(id)
	confirmAskers.Store(id, fn)
	return func() {
		if prev == nil {
			confirmAskers.Delete(id)
			return
		}
		confirmAskers.Store(id, prev)
	}
}

func currentConfirmAsk() ConfirmAskFunc {
	v, ok := confirmAskers.Load(goroutineID())
	if !ok {
		return nil
	}
	fn, _ := v.(ConfirmAskFunc)
	return fn
}

func goroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	// "goroutine 123 [running]:\n..."
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	i := bytes.IndexByte(b, ' ')
	if i <= 0 {
		return 0
	}
	id, _ := strconv.ParseUint(string(b[:i]), 10, 64)
	return id
}
