package task

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tokamak-network/trh-backend/pkg/taskmanager"
)

type TaskHandler struct {
	TaskManager *taskmanager.TaskManager
}

func NewTaskHandler(taskManager *taskmanager.TaskManager) *TaskHandler {
	return &TaskHandler{
		TaskManager: taskManager,
	}
}

// GetTaskProgress godoc
// @Summary Get task progress
// @Description Get the current status and progress of a background task
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} taskmanager.TaskProgress
// @Failure 404 {object} map[string]string "Task not found"
// @Router /tasks/{id} [get]
func (h *TaskHandler) GetTaskProgress(c *gin.Context) {
	taskId := c.Param("id")
	if taskId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}

	progress, exists := h.TaskManager.GetTaskProgress(taskId)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or expired"})
		return
	}

	c.JSON(http.StatusOK, progress)
}
