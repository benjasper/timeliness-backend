package tasks

import (
	"github.com/benjasper/project-tasks/pkg/communication"
	"github.com/benjasper/project-tasks/pkg/logger"
	"net/http"
)

type Handler struct {
	TaskService  TaskServiceInterface
	Logger       logger.Interface
	ErrorManager *communication.ErrorResponseManager
}

func (handler *Handler) HandleTaskAdd(writer http.ResponseWriter, request *http.Request) {
	panic("Not implemented yet")
}
