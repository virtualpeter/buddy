package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"buddy/internal/buddy"
)

var updateNotice atomic.Value // buddy.UpdateNotice

func publishUpdateNotice(n buddy.UpdateNotice) {
	updateNotice.Store(n)
}

func currentUpdateNotice() buddy.UpdateNotice {
	if v := updateNotice.Load(); v != nil {
		if n, ok := v.(buddy.UpdateNotice); ok {
			return n
		}
	}
	return buddy.UpdateNotice{Current: chromeVersion}
}

func startReleaseCheck(ctx context.Context) {
	current := chromeVersion
	gui := true
	if cfg := buddy.GetActiveConfig(); cfg != nil {
		gui = cfg.CheckUpdatesGUI()
	}
	if !gui {
		publishUpdateNotice(buddy.UpdateNotice{Current: current, Checked: true})
		return
	}
	cached := buddy.CachedUpdateNotice(current)
	publishUpdateNotice(cached)
	if cached.Checked {
		return
	}
	go func() {
		checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		publishUpdateNotice(buddy.RefreshUpdateCheck(checkCtx, current))
	}()
}

func handleUpdateAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(currentUpdateNotice())
}
