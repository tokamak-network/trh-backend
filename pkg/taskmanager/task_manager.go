package taskmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"go.uber.org/zap"
)

// TaskProgress represents the current status of a running task
type TaskProgress struct {
	ID         string  `json:"id"`
	Status     string  `json:"status"`     // "pending", "running", "completed", "failed"
	Percentage float64 `json:"percentage"` // 0.0 to 100.0
	Message    string  `json:"message"`    // Current status message
	Error      string  `json:"error,omitempty"`
	Result     any     `json:"result,omitempty"`
	UpdatedAt  string  `json:"updatedAt"`
	StartedAt  string  `json:"startedAt"`
}

type managedTask struct {
	id     string
	task   entities.Task
	ctx    context.Context
	cancel context.CancelFunc

	// For progress tracking
	isProgressTask bool
	progressTask   func(ctx context.Context, updateProgress func(string, float64))
}

type TaskManager struct {
	tasks      chan *managedTask
	numWorkers int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	// For cancellation
	taskLock    sync.RWMutex
	activeTasks map[string]*managedTask

	// Progress storage
	progressMu sync.RWMutex
	progress   map[string]*TaskProgress
}

func NewTaskManager(numWorkers int) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	tm := &TaskManager{
		tasks:       make(chan *managedTask, 100), // Default buffer size
		numWorkers:  numWorkers,
		ctx:         ctx,
		cancel:      cancel,
		activeTasks: make(map[string]*managedTask),
		progress:    make(map[string]*TaskProgress),
	}
	tm.Start()
	return tm
}

func (tm *TaskManager) Start() {
	for i := 0; i < tm.numWorkers; i++ {
		tm.wg.Add(1)
		go tm.worker(i)
	}
	logger.Info("Task manager started", zap.Int("workers", tm.numWorkers))
}

func (tm *TaskManager) worker(id int) {
	defer tm.wg.Done()
	for {
		select {
		case <-tm.ctx.Done():
			logger.Info("Worker exiting", zap.Int("workerID", id))
			return
		case task, ok := <-tm.tasks:
			if !ok {
				logger.Info("Task channel closed", zap.Int("workerID", id))
				return
			}
			logger.Info("Starting task", zap.String("task_id", task.id), zap.Int("worker_id", id))

			// Register active task
			tm.taskLock.Lock()
			tm.activeTasks[task.id] = task
			tm.taskLock.Unlock()

			if task.isProgressTask {
				tm.updateTaskStatus(task.id, "running", 0.0, "Started")
				func() {
					defer func() {
						if r := recover(); r != nil {
							err := fmt.Errorf("task panicked: %v", r)
							logger.Error("Task panic", zap.String("task_id", task.id), zap.Error(err))
							tm.updateTaskStatus(task.id, "failed", 0.0, fmt.Sprintf("Task panicked: %v", r))
						}
					}()
					task.progressTask(task.ctx, func(msg string, pct float64) {
						tm.updateTaskStatus(task.id, "running", pct, msg)
					})
					// Only mark as completed if no panic occurred
					if task.ctx.Err() == nil { // Check if context was not cancelled
						tm.updateTaskStatus(task.id, "completed", 100.0, "Completed")
					} else {
						tm.updateTaskStatus(task.id, "failed", 0.0, "Task cancelled")
					}
				}()
			} else {
				func() {
					defer func() {
						if r := recover(); r != nil {
							err := fmt.Errorf("task panicked: %v", r)
							logger.Error("Task panic", zap.String("task_id", task.id), zap.Error(err))
						}
					}()
					task.task(task.ctx)
				}()
			}

			// Unregister active task
			tm.taskLock.Lock()
			delete(tm.activeTasks, task.id)
			tm.taskLock.Unlock()

			logger.Info("Task completed", zap.String("task_id", task.id))
			task.cancel()
		}
	}
}

// AddTask adds a task with a unique ID
func (tm *TaskManager) AddTask(id string, task entities.Task) {
	// If ID is empty, generate one
	if id == "" {
		id = uuid.New().String()
	}

	ctx, cancel := context.WithCancel(context.Background()) // Use a new context for each task

	mt := &managedTask{
		id:             id,
		task:           task,
		ctx:            ctx,
		cancel:         cancel,
		isProgressTask: false,
	}

	select {
	case tm.tasks <- mt:
		logger.Debug("Task added successfully", zap.String("taskID", id))
	default:
		logger.Warn("Task queue is full, dropping task", zap.String("taskID", id))
		cancel() // Cancel context if dropped
	}
}

// AddTaskWithProgress adds a task that reports progress
func (tm *TaskManager) AddTaskWithProgress(id string, task func(ctx context.Context, updateProgress func(string, float64))) {
	// If ID is empty, generate one
	if id == "" {
		id = uuid.New().String()
	}

	ctx, cancel := context.WithCancel(context.Background()) // Use a new context for each task

	now := time.Now().Format(time.RFC3339)

	// Initialize progress
	tm.progressMu.Lock()
	tm.progress[id] = &TaskProgress{
		ID:         id,
		Status:     "pending",
		Percentage: 0.0,
		Message:    "Queued",
		UpdatedAt:  now,
		StartedAt:  now,
	}
	tm.progressMu.Unlock()

	mt := &managedTask{
		id:             id,
		progressTask:   task,
		ctx:            ctx,
		cancel:         cancel,
		isProgressTask: true,
	}

	select {
	case tm.tasks <- mt:
		logger.Debug("Progress task added successfully", zap.String("taskID", id))
	default:
		logger.Warn("Progress task queue is full, dropping task", zap.String("taskID", id))
		cancel()
		// Clean up progress if task is dropped
		tm.progressMu.Lock()
		delete(tm.progress, id)
		tm.progressMu.Unlock()
	}
}

func (tm *TaskManager) GetTaskProgress(id string) (*TaskProgress, bool) {
	tm.progressMu.RLock()
	defer tm.progressMu.RUnlock()
	p, ok := tm.progress[id]
	return p, ok
}

func (tm *TaskManager) SetTaskResult(id string, result any) {
	tm.progressMu.Lock()
	defer tm.progressMu.Unlock()

	if p, ok := tm.progress[id]; ok {
		p.Result = result
		p.UpdatedAt = time.Now().Format(time.RFC3339)
	}
}

func (tm *TaskManager) updateTaskStatus(id, status string, pct float64, msg string) {
	tm.progressMu.Lock()
	defer tm.progressMu.Unlock()

	if p, ok := tm.progress[id]; ok {
		p.Status = status
		p.Percentage = pct
		p.Message = msg
		p.UpdatedAt = time.Now().Format(time.RFC3339)
	}
}

// StopTask stops a task by ID (if it's currently running)
// This method is kept for compatibility but its functionality is limited
// as tasks are processed from a channel and not actively managed in a map.
// It can only cancel a task if it's currently running and responsive to context cancellation.
func (tm *TaskManager) StopTask(id string) {
	tm.taskLock.RLock()
	mt, exists := tm.activeTasks[id]
	tm.taskLock.RUnlock()

	if exists {
		logger.Info("Cancelling task", zap.String("taskID", id))
		mt.cancel()
		// Note: The worker will remove it from activeTasks when it finishes
	} else {
		logger.Debug("Task not found or already finished", zap.String("taskID", id))
	}
}

// IsTaskRunning checks if a task is currently running
func (tm *TaskManager) IsTaskRunning(id string) bool {
	tm.taskLock.RLock()
	defer tm.taskLock.RUnlock()
	_, exists := tm.activeTasks[id]
	return exists
}

// Stop stops all workers and tasks
func (tm *TaskManager) Stop() {
	logger.Info("Stopping TaskManager...")
	tm.cancel()
	tm.wg.Wait()
	close(tm.tasks)
	logger.Info("All workers stopped")
}
