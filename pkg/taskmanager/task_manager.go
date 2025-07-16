package taskmanager

import (
	"context"
	"sync"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"go.uber.org/zap"
)

type managedTask struct {
	id     string
	task   entities.Task
	ctx    context.Context
	cancel context.CancelFunc
}

type TaskManager struct {
	tasks       chan *managedTask
	numWorkers  int
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	taskLock    sync.RWMutex // Changed to RWMutex for better performance
	activeTasks map[string]*managedTask
}

func NewTaskManager(numWorkers int, bufferSize int) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskManager{
		tasks:       make(chan *managedTask, bufferSize),
		numWorkers:  numWorkers,
		ctx:         ctx,
		cancel:      cancel,
		activeTasks: make(map[string]*managedTask, bufferSize), // Pre-allocate with capacity
	}
}

func (tm *TaskManager) Start() {
	for i := 0; i < tm.numWorkers; i++ { // Fixed range loop
		tm.wg.Add(1)
		go func(workerID int) {
			defer tm.wg.Done()
			for {
				select {
				case <-tm.ctx.Done():
					logger.Info("Worker exiting", zap.Int("workerID", workerID))
					return
				case mt, ok := <-tm.tasks:
					if !ok {
						logger.Info("Task channel closed", zap.Int("workerID", workerID))
						return
					}

					tm.taskLock.Lock()
					tm.activeTasks[mt.id] = mt
					tm.taskLock.Unlock()

					logger.Info("Running task", zap.Int("workerID", workerID), zap.String("taskID", mt.id))
					mt.task(mt.ctx)

					tm.taskLock.Lock()
					delete(tm.activeTasks, mt.id)
					tm.taskLock.Unlock()
				}
			}
		}(i)
	}
}

// AddTask adds a task with a unique ID
func (tm *TaskManager) AddTask(id string, task entities.Task) {
	ctx, cancel := context.WithCancel(tm.ctx)
	mt := &managedTask{
		id:     id,
		task:   task,
		ctx:    ctx,
		cancel: cancel,
	}

	select {
	case tm.tasks <- mt:
		logger.Debug("Task added successfully", zap.String("taskID", id))
	default:
		logger.Warn("Task queue is full, dropping task", zap.String("taskID", id))
	}
}

// StopTask stops a task by ID (if it's currently running)
func (tm *TaskManager) StopTask(id string) {
	tm.taskLock.RLock()
	mt, exists := tm.activeTasks[id]
	tm.taskLock.RUnlock()

	if exists {
		logger.Info("Cancelling task", zap.String("taskID", id))
		mt.cancel()

		tm.taskLock.Lock()
		delete(tm.activeTasks, id)
		tm.taskLock.Unlock()
	} else {
		logger.Debug("Task not found or already finished", zap.String("taskID", id))
	}
}

// Stop stops all workers and tasks
func (tm *TaskManager) Stop() {
	logger.Info("Stopping TaskManager...")
	tm.cancel()
	tm.wg.Wait()
	close(tm.tasks)
	logger.Info("All workers stopped")
}
