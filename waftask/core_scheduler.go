// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waftask

import (
	"HomeWaf/common/tasklog"
	"HomeWaf/common/zlog"
	"HomeWaf/enums"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	_ "time/tzdata"
)

// TaskScheduler 用于封装任务调度器
type TaskScheduler struct {
	Scheduler gocron.Scheduler
	Registry  *TaskRegistry
	// 任务标签映射，用于管理任务的卸载和重载
	taskTags map[string]string // key: taskMethod, value: jobTag
	mu       sync.RWMutex      // 保护 taskTags 的并发访问
}

// NewTaskScheduler 创建新的任务调度器
func NewTaskScheduler(registry *TaskRegistry) *TaskScheduler {
	timezone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	scheduler, err := gocron.NewScheduler(gocron.WithLocation(timezone))
	if err != nil {
		panic(err)
	}
	return &TaskScheduler{
		Scheduler: scheduler,
		Registry:  registry,
		taskTags:  make(map[string]string),
	}
}

// ScheduleTask 用于调度任务
// unit 表示单位："second"、"minute"、"hour"、"day"
// interval 表示时间间隔
// at 具体的时间
// taskMethod 是具体要执行的任务
// taskDaysOfTheWeek 如果是周级别 此处传入周几
func (ts *TaskScheduler) ScheduleTask(unit string, interval int, at string, taskMethod string, taskDaysOfTheWeek string) error {
	if interval < 1 {
		return fmt.Errorf("task interval must be positive")
	}
	jobTag := fmt.Sprintf("task_%s", taskMethod)
	var definition gocron.JobDefinition
	options := []gocron.JobOption{gocron.WithTags(jobTag)}
	switch unit {
	case enums.TASK_SECOND:
		definition = gocron.DurationJob(time.Duration(interval) * time.Second)
	case enums.TASK_MIN:
		definition = gocron.DurationJob(time.Duration(interval) * time.Minute)
	case enums.TASK_HOUR:
		definition = gocron.DurationJob(time.Duration(interval) * time.Hour)
	case enums.TASK_DAY, enums.TASK_WEEKLY:
		var times []gocron.AtTime
		for _, value := range strings.FieldsFunc(at, func(c rune) bool { return c == ',' || c == ';' }) {
			parsed, err := time.Parse("15:04:05", strings.TrimSpace(value))
			if err != nil {
				parsed, err = time.Parse("15:04", strings.TrimSpace(value))
			}
			if err != nil {
				return fmt.Errorf("invalid task time: %w", err)
			}
			times = append(times, gocron.NewAtTime(uint(parsed.Hour()), uint(parsed.Minute()), uint(parsed.Second())))
		}
		if len(times) == 0 {
			return fmt.Errorf("task time is required")
		}
		if unit == enums.TASK_DAY {
			definition = gocron.DailyJob(uint(interval), gocron.NewAtTimes(times[0], times[1:]...))
		} else {
			day, err := strconv.Atoi(strings.TrimSpace(taskDaysOfTheWeek))
			if err != nil || day < 0 || day > 6 {
				return fmt.Errorf("invalid weekday: %s", taskDaysOfTheWeek)
			}
			definition = gocron.WeeklyJob(uint(interval), gocron.NewWeekdays(time.Weekday(day)), gocron.NewAtTimes(times[0], times[1:]...))
		}
	default:
		return fmt.Errorf("unsupported unit: %s", unit)
	}
	if unit == enums.TASK_SECOND || unit == enums.TASK_MIN || unit == enums.TASK_HOUR {
		options = append(options, gocron.WithStartAt(gocron.WithStartImmediately()))
	}
	job, err := ts.Scheduler.NewJob(definition, gocron.NewTask(func() { ts.Registry.ExecuteTask(taskMethod) }), options...)

	if err != nil {
		return fmt.Errorf("failed to schedule task: %v", err)
	}

	// 保存任务标签映射
	ts.mu.Lock()
	ts.taskTags[taskMethod] = jobTag
	ts.mu.Unlock()

	// 按任务执行频率设置日志策略（速率限制 + 保留天数）
	applyTaskLogPolicy(taskMethod, unit, interval)

	zlog.Debug(fmt.Sprintf("Task scheduled: %v every %d %s\n", job, interval, unit))
	return nil
}

// applyTaskLogPolicy 根据任务执行频率，为其设置日志速率限制和保留策略：
//   - 秒级任务：INFO/DEBUG 每 60s 最多一次；日志仅保留 1 天
//   - 分钟级（间隔 ≤ 2 分钟）：INFO/DEBUG 每 30s 最多一次；保留天数使用全局默认
//   - 其他任务：不限制，保留天数使用全局默认
func applyTaskLogPolicy(taskMethod string, unit string, interval int) {
	if tasklog.GlobalTaskLogManager == nil {
		return
	}
	switch unit {
	case enums.TASK_SECOND:
		tasklog.GlobalTaskLogManager.SetTaskLogPolicy(taskMethod, 60*time.Second)
		tasklog.GlobalTaskLogManager.SetTaskRetainDays(taskMethod, 1)
	case enums.TASK_MIN:
		if interval <= 2 {
			tasklog.GlobalTaskLogManager.SetTaskLogPolicy(taskMethod, 30*time.Second)
		}
	}
}
func (ts *TaskScheduler) Start() {
	ts.Scheduler.Start()
}
func (ts *TaskScheduler) Stop() {
	if ts != nil {
		_ = ts.Scheduler.Shutdown()
	}
}

func (ts *TaskScheduler) RunManual(taskMethod string) {
	ts.Registry.ExecuteTask(taskMethod)
}

// UnscheduleTask 卸载指定的任务
func (ts *TaskScheduler) UnscheduleTask(taskMethod string) error {
	ts.mu.RLock()
	jobTag, exists := ts.taskTags[taskMethod]
	ts.mu.RUnlock()

	if !exists {
		zlog.Warn("任务未找到，无法卸载", "taskMethod", taskMethod)
		return fmt.Errorf("任务 %s 未找到", taskMethod)
	}

	// 通过标签移除任务
	ts.Scheduler.RemoveByTags(jobTag)

	// 从映射中移除
	ts.mu.Lock()
	delete(ts.taskTags, taskMethod)
	ts.mu.Unlock()

	zlog.Info("任务卸载成功", "taskMethod", taskMethod)
	return nil
}

// RescheduleTask 重新调度任务（先卸载再加载）
func (ts *TaskScheduler) RescheduleTask(unit string, interval int, at string, taskMethod string, taskDaysOfTheWeek string) error {
	// 先尝试卸载旧任务（如果存在）
	ts.mu.RLock()
	_, exists := ts.taskTags[taskMethod]
	ts.mu.RUnlock()

	if exists {
		if err := ts.UnscheduleTask(taskMethod); err != nil {
			zlog.Warn("卸载旧任务时出错，继续重新调度", "taskMethod", taskMethod, "error", err.Error())
		}
	}

	// 重新调度任务
	if err := ts.ScheduleTask(unit, interval, at, taskMethod, taskDaysOfTheWeek); err != nil {
		return fmt.Errorf("重新调度任务失败: %w", err)
	}

	zlog.Info("任务重新调度成功", "taskMethod", taskMethod, "unit", unit, "interval", interval, "at", at)
	return nil
}
