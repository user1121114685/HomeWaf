// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafdb

import (
	"sync"
	"time"

	"gorm.io/gorm"
)

// 归档分片连接缓存。
//
// shardMu 保护连接、最后使用时间以及等待回收的连接。缓存限制活跃分片数和空闲时间。
//
// 淘汰出缓存的连接不立刻关：同一次请求里可能还握着它（扇出先统计、后取数），
// 立刻关会让后半段拿到 database is closed。放进 retired，过 shardRetireGrace 再关。
const (
	shardCacheMax    = 8
	shardIdleTTL     = 30 * time.Minute
	shardRetireGrace = 2 * time.Minute
)

var (
	shardMu      sync.Mutex
	shardDBs     = map[string]*gorm.DB{}
	shardLastUse = map[string]time.Time{}
	shardRetired = map[string][]*gorm.DB{}
)

// getShardDB 取一个已打开的分片连接并刷新它的使用时间；没有则返回 nil。
func getShardDB(name string) *gorm.DB {
	shardMu.Lock()
	defer shardMu.Unlock()
	db := shardDBs[name]
	if db != nil {
		shardLastUse[name] = time.Now()
	}
	return db
}

// putShardDB 登记一个新打开的连接。若同名连接已被别的协程抢先放入，
// 返回那一个并让调用方关掉自己开的，避免同一个文件握两份句柄。
func putShardDB(name string, db *gorm.DB) (kept *gorm.DB, duplicated bool) {
	shardMu.Lock()
	defer shardMu.Unlock()
	if exist := shardDBs[name]; exist != nil {
		shardLastUse[name] = time.Now()
		return exist, true
	}
	shardDBs[name] = db
	shardLastUse[name] = time.Now()
	// 重新打开的分片（例如文件被拷回来）重新选层
	forgetShardTiers(name)
	evictLocked()
	return db, false
}

// closeShardDB 关闭并移除一个分片连接。归档清理删文件前必须先调用，
// 否则 Windows 下文件被占用删不掉。
// 宽限期里还没关的同名连接一并关掉。
func closeShardDB(name string) {
	shardMu.Lock()
	db := shardDBs[name]
	delete(shardDBs, name)
	delete(shardLastUse, name)
	retired := shardRetired[name]
	delete(shardRetired, name)
	shardMu.Unlock()

	closeConnection(name, db)
	for _, r := range retired {
		closeConnection(name, r)
	}
	forgetShardTiers(name)
}

// retireLocked 把一个连接移出缓存，宽限期后再关。调用方持锁。
func retireLocked(name string) {
	db := shardDBs[name]
	delete(shardDBs, name)
	delete(shardLastUse, name)
	if db == nil {
		return
	}
	shardRetired[name] = append(shardRetired[name], db)
	time.AfterFunc(shardRetireGrace, func() {
		shardMu.Lock()
		list := shardRetired[name]
		kept := list[:0]
		found := false
		for _, r := range list {
			if r == db {
				found = true
				continue
			}
			kept = append(kept, r)
		}
		if len(kept) == 0 {
			delete(shardRetired, name)
		} else {
			shardRetired[name] = kept
		}
		shardMu.Unlock()
		// 已被 closeShardDB 提前关掉的不再关第二次
		if found {
			closeConnection(name, db)
		}
	})
}

// snapshotShardDBs 返回当前连接的快照，供监控之类只读遍历使用，
// 免得调用方直接遍历 map 时撞上并发写。
func snapshotShardDBs() map[string]*gorm.DB {
	shardMu.Lock()
	defer shardMu.Unlock()
	out := make(map[string]*gorm.DB, len(shardDBs))
	for k, v := range shardDBs {
		out[k] = v
	}
	return out
}

// evictLocked 在持锁状态下淘汰：先按空闲时间清，再按数量清最久未用的。
// 实际关闭放到锁外做，关连接可能阻塞。
func evictLocked() {
	var victims []string
	now := time.Now()
	for name, last := range shardLastUse {
		if now.Sub(last) > shardIdleTTL {
			victims = append(victims, name)
		}
	}
	for _, name := range victims {
		retireLocked(name)
	}

	for len(shardDBs) > shardCacheMax {
		oldest := ""
		var oldestAt time.Time
		for name, last := range shardLastUse {
			if oldest == "" || last.Before(oldestAt) {
				oldest, oldestAt = name, last
			}
		}
		if oldest == "" {
			return
		}
		retireLocked(oldest)
	}
}

// Workers call this after draining their users of the shard cache.
func closeAllShardDBs() {
	shardMu.Lock()
	closing := shardRetired
	for name, db := range shardDBs {
		closing[name] = append(closing[name], db)
	}
	for name := range closing {
		forgetShardTiers(name)
	}
	clear(shardDBs)
	clear(shardLastUse)
	shardRetired = make(map[string][]*gorm.DB)
	shardMu.Unlock()

	for name, connections := range closing {
		for _, db := range connections {
			closeConnection(name, db)
		}
	}
}
