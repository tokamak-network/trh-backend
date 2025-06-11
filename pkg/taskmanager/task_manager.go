package taskmanager

import (
	"context"
	"log"
	"sync"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
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
	taskLock    sync.Mutex
	activeTasks map[string]*managedTask
}

func NewTaskManager(numWorkers int, bufferSize int) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskManager{
		tasks:       make(chan *managedTask, bufferSize),
		numWorkers:  numWorkers,
		ctx:         ctx,
		cancel:      cancel,
		activeTasks: make(map[string]*managedTask),
	}
}

func (tm *TaskManager) Start() {
	for i := range tm.numWorkers {
		tm.wg.Add(1)
		go func(workerID int) {
			defer tm.wg.Done()
			for {
				select {
				case <-tm.ctx.Done():
					log.Printf("Worker %d exiting", workerID)
					return
				case mt, ok := <-tm.tasks:
					if !ok {
						log.Printf("Worker %d: task channel closed", workerID)
						return
					}

					tm.taskLock.Lock()
					tm.activeTasks[mt.id] = mt
					tm.taskLock.Unlock()

					log.Printf("Worker %d running task %s", workerID, mt.id)
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
	tm.tasks <- mt
}

// StopTask stops a task by ID (if it's currently running)
func (tm *TaskManager) StopTask(id string) {
	tm.taskLock.Lock()
	defer tm.taskLock.Unlock()

	if mt, exists := tm.activeTasks[id]; exists {
		log.Printf("Cancelling task %s", id)
		mt.cancel()
		delete(tm.activeTasks, id)
	} else {
		log.Printf("Task %s not found or already finished", id)
	}
}

// Stop stops all workers and tasks
func (tm *TaskManager) Stop() {
	log.Println("Stopping TaskManager...")
	tm.cancel()
	tm.wg.Wait()
	close(tm.tasks)
	log.Println("All workers stopped.")
}
