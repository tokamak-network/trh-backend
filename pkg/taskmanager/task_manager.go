package taskmanager

import (
	"context"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"log"
	"sync"
)

type TaskManager struct {
	tasks      chan entities.Task
	numWorkers int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewTaskManager(numWorkers int, bufferSize int) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskManager{
		tasks:      make(chan entities.Task, bufferSize),
		numWorkers: numWorkers,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (tm *TaskManager) Start() {
	for i := 0; i < tm.numWorkers; i++ {
		tm.wg.Add(1)
		go func(workerID int) {
			defer tm.wg.Done()
			for {
				select {
				case <-tm.ctx.Done():
					log.Printf("Worker %d exiting", workerID)
					return
				case task := <-tm.tasks:
					log.Printf("Worker %d running task", workerID)
					task()
				}
			}
		}(i)
	}
}

func (tm *TaskManager) AddTask(task entities.Task) {
	tm.tasks <- task
}

func (tm *TaskManager) Stop() {
	tm.cancel()
	close(tm.tasks)
	tm.wg.Wait()
	log.Println("All workers stopped.")
}
