// Package acmelease coordinates certificate work with local rolling restarts.
package acmelease

import (
	"context"
	"path/filepath"

	"HomeWaf/internal/platform/utils/filelease"
)

type Guard struct{ admission, activity *filelease.Lease }

func (g *Guard) Release() {
	if g == nil {
		return
	}
	if g.activity != nil {
		g.activity.Release()
	}
	if g.admission != nil {
		g.admission.Release()
	}
}

// A job registers its shared activity while holding admission. A restart holds
// admission until handoff finishes, so no new job can slip into the old worker.
func AcquireJob(ctx context.Context, dataDir string) (*Guard, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	gate, err := filelease.AcquireExclusive(ctx, filepath.Join(dataDir, "ssl-admission.lock"))
	if err != nil {
		return nil, err
	}
	defer gate.Release()
	active, err := filelease.AcquireShared(ctx, filepath.Join(dataDir, "ssl-activity.lock"))
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		active.Release()
		return nil, err
	}
	return &Guard{activity: active}, nil
}

func AcquireRestart(ctx context.Context, dataDir string) (*Guard, error) {
	return acquireRestart(ctx, dataDir, nil)
}

func acquireRestart(ctx context.Context, dataDir string, admitted func()) (*Guard, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	gate, err := filelease.AcquireExclusive(ctx, filepath.Join(dataDir, "ssl-admission.lock"))
	if err != nil {
		return nil, err
	}
	if admitted != nil {
		admitted()
	}
	active, err := filelease.AcquireExclusive(ctx, filepath.Join(dataDir, "ssl-activity.lock"))
	if err != nil {
		gate.Release()
		return nil, err
	}
	guard := &Guard{admission: gate, activity: active}
	if err := ctx.Err(); err != nil {
		guard.Release()
		return nil, err
	}
	return guard, nil
}
