package cron

import (
	"context"
	"sync"
	"time"

	"github.com/jettjia/XiaoQinglong/runner/pkg/logger"
)

// ========== Scheduler Types ==========

// TaskHandler handles task firing events
type TaskHandler interface {
	// OnTaskFired is called when a task fires
	OnTaskFired(taskID string, prompt string)
	// OnTaskError is called when a task execution fails
	OnTaskError(taskID string, err error)
}

// TaskFiredFunc is a function adapter for TaskHandler
type TaskFiredFunc func(taskID string, prompt string)

// OnTaskFired implements TaskHandler
func (f TaskFiredFunc) OnTaskFired(taskID string, prompt string) {
	f(taskID, prompt)
}

// OnTaskError implements TaskHandler
func (f TaskFiredFunc) OnTaskError(taskID string, err error) {}

// Scheduler manages cron task execution
type Scheduler struct {
	mu         sync.RWMutex
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
	ticker     *time.Ticker
	handler    TaskHandler
	projectDir string
}

// ========== Scheduler Configuration ==========

// JitterConfig holds jitter configuration for task firing
type JitterConfig struct {
	RecurringFrac    float64 // Fraction of interval for recurring task delay
	RecurringCapMs   int64   // Max delay for recurring tasks
	OneShotMaxMs     int64   // Max early fire for one-shot tasks
	OneShotFloorMs   int64   // Min early fire for one-shot tasks
	OneShotMinuteMod int     // Minute mod gate for one-shot jitter
}

var DefaultJitterConfig = JitterConfig{
	RecurringFrac:    0.1,
	RecurringCapMs:    15 * 60 * 1000, // 15 minutes
	OneShotMaxMs:      90 * 1000,      // 90 seconds
	OneShotFloorMs:    0,
	OneShotMinuteMod:  30,
}

// ========== Scheduler Management ==========

var (
	defaultScheduler     *Scheduler
	defaultSchedulerOnce sync.Once
	defaultSchedulerMu   sync.RWMutex
)

// GetScheduler returns the default scheduler instance
func GetScheduler() *Scheduler {
	defaultSchedulerMu.RLock()
	if defaultScheduler != nil {
		defaultSchedulerMu.RUnlock()
		return defaultScheduler
	}
	defaultSchedulerMu.RUnlock()

	defaultSchedulerOnce.Do(func() {
		defaultSchedulerMu.Lock()
		defer defaultSchedulerMu.Unlock()
		if defaultScheduler == nil {
			defaultScheduler = NewScheduler(".")
			logger.Infof("[CronScheduler] Created default scheduler")
		}
	})
	return defaultScheduler
}

// NewScheduler creates a new scheduler
func NewScheduler(projectDir string) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		running:    false,
		ctx:        ctx,
		cancel:     cancel,
		projectDir: projectDir,
	}
}

// SetHandler sets the task handler for when tasks fire
func (s *Scheduler) SetHandler(handler TaskHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		logger.Warnf("[CronScheduler] Already running")
		return
	}

	s.running = true
	s.ticker = time.NewTicker(time.Second)
	go s.run()

	logger.Infof("[CronScheduler] Started")
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	s.cancel()

	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
	}

	logger.Infof("[CronScheduler] Stopped")
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// ========== Scheduler Loop ==========

func (s *Scheduler) run() {
	logger.Infof("[CronScheduler] Scheduler loop started")

	for {
		select {
		case <-s.ctx.Done():
			logger.Infof("[CronScheduler] Scheduler loop exited")
			return
		case <-s.ticker.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() {
	now := time.Now()

	// Get all tasks
	tasks := ListAllTasks(s.projectDir)
	if len(tasks) == 0 {
		return
	}

	// Find tasks that should fire
	var firedTasks []*CronTask
	for _, task := range tasks {
		nextFire, err := s.getNextFireTime(task, now)
		if err != nil {
			logger.Warnf("[CronScheduler] Task %s: failed to get next fire time: %v", task.ID, err)
			continue
		}

		if nextFire == nil {
			continue
		}

		// Check if task should fire now
		if nextFire.Before(now) || nextFire.Equal(now) {
			firedTasks = append(firedTasks, task)
		}
	}

	if len(firedTasks) == 0 {
		return
	}

	// Fire tasks
	logger.Infof("[CronScheduler] Firing %d tasks", len(firedTasks))

	var firedIDs []string
	for _, task := range firedTasks {
		// Fire the task
		logger.Infof("[CronScheduler] Firing task %s: %s", task.ID, task.Prompt)

		// Call the handler
		s.mu.RLock()
		h := s.handler
		s.mu.RUnlock()

		if h != nil {
			h.OnTaskFired(task.ID, task.Prompt)
		}

		firedIDs = append(firedIDs, task.ID)

		// Update last fired time for recurring tasks
		if task.Recurring && task.Durable {
			firedAt := now.UnixMilli()
			MarkTasksFired([]string{task.ID}, firedAt, s.projectDir)
		}
	}

	// Remove one-shot tasks (they fire once then delete)
	if len(firedIDs) > 0 {
		oneShotIDs := make([]string, 0)
		for _, task := range firedTasks {
			if !task.Recurring {
				oneShotIDs = append(oneShotIDs, task.ID)
			}
		}
		if len(oneShotIDs) > 0 {
			if err := RemoveTasks(oneShotIDs, s.projectDir); err != nil {
				logger.Errorf("[CronScheduler] Remove one-shot tasks failed: %v", err)
			}
		}
	}
}

// getNextFireTime calculates the next fire time for a task
func (s *Scheduler) getNextFireTime(task *CronTask, from time.Time) (*time.Time, error) {
	fields := ParseCronExpression(task.Cron)
	if fields == nil {
		return nil, nil
	}

	// Use lastFiredAt as anchor if available, otherwise use CreatedAt
	var anchor time.Time
	if task.LastFiredAt != nil {
		anchor = time.UnixMilli(*task.LastFiredAt)
	} else {
		anchor = time.UnixMilli(task.CreatedAt)
	}

	// For recurring tasks, add jitter
	if task.Recurring {
		// Compute next run from anchor
		next := ComputeNextCronRun(fields, anchor)
		if next == nil {
			return nil, nil
		}

		// Compute the run after that to determine interval
		interval := int64(0)
		if task.LastFiredAt != nil {
			nextAfter := ComputeNextCronRun(fields, *next)
			if nextAfter != nil {
				interval = nextAfter.Sub(*next).Milliseconds()
			}
		}

		if interval > 0 {
			// Apply jitter proportional to interval
			jitter := int64(float64(interval) * DefaultJitterConfig.RecurringFrac * jitterFrac(task.ID))
			if jitter > DefaultJitterConfig.RecurringCapMs {
				jitter = DefaultJitterConfig.RecurringCapMs
			}
			fireTime := next.Add(time.Duration(jitter) * time.Millisecond)
			return &fireTime, nil
		}

		return next, nil
	}

	// One-shot tasks: compute next run, apply backward jitter if on round minute
	next := ComputeNextCronRun(fields, anchor)
	if next == nil {
		return nil, nil
	}

	// Apply backward jitter for one-shot tasks on :00 or :30
	if next.Minute()%DefaultJitterConfig.OneShotMinuteMod == 0 {
		lead := DefaultJitterConfig.OneShotFloorMs +
			int64(float64(DefaultJitterConfig.OneShotMaxMs-DefaultJitterConfig.OneShotFloorMs)*jitterFrac(task.ID))
		fireTime := next.Add(-time.Duration(lead) * time.Millisecond)
		// Don't fire before anchor
		if fireTime.Before(anchor) {
			return &anchor, nil
		}
		return &fireTime, nil
	}

	return next, nil
}

// jitterFrac computes a stable fraction [0,1) from task ID for jitter
func jitterFrac(taskID string) float64 {
	var sum uint64
	for _, c := range taskID {
		if c >= '0' && c <= '9' {
			sum = sum*16 + uint64(c-'0')
		} else if c >= 'a' && c <= 'f' {
			sum = sum*16 + uint64(c-'a'+10)
		} else if c >= 'A' && c <= 'F' {
			sum = sum*16 + uint64(c-'A'+10)
		}
	}
	return float64(sum) / float64(0x1_0000_0000)
}

// ========== Task Queries ==========

// NextCronRunMs computes the next fire time in epoch ms
func NextCronRunMs(cron string, fromMs int64) (int64, error) {
	fields := ParseCronExpression(cron)
	if fields == nil {
		return 0, nil
	}

	from := time.UnixMilli(fromMs)
	next := ComputeNextCronRun(fields, from)
	if next == nil {
		return 0, nil
	}

	return next.UnixMilli(), nil
}
