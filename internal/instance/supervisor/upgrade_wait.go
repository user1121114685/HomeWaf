package supervisor

import (
	"context"
	"fmt"
	"time"
)

// TriggerUpgradeAndWait keeps the caller's certificate guard held through the
// entire handoff, including the old worker's exit, rather than only until ACK.
func TriggerUpgradeAndWait(ctx context.Context, dataDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	before := loadPrevStateFile(dataDir)
	if before == nil {
		return fmt.Errorf("无法读取运行中的 Supervisor 状态")
	}
	if before.Upgrading {
		return fmt.Errorf("已有进程重载正在进行")
	}
	if err := TriggerUpgrade(dataDir); err != nil {
		return err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		state := loadPrevStateFile(dataDir)
		if state != nil && state.UpgradeSequence > before.UpgradeSequence && !state.Upgrading {
			if state.ActivePID == before.ActivePID {
				return fmt.Errorf("新进程未能接管，原进程继续服务")
			}
			oldPresent, activePresent := false, false
			for _, pid := range state.PIDs {
				if pid == before.ActivePID {
					oldPresent = true
				}
				if pid == state.ActivePID && pid > 0 {
					activePresent = true
				}
			}
			if !oldPresent && activePresent && (state.ActiveState == "active" || state.ActiveState == "active-candidate") {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
