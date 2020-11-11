package main

import (
	"context"
	"fmt"
	"github.com/benjasper/project-tasks/pkg/auth"
	"github.com/benjasper/project-tasks/pkg/communication"
	"github.com/benjasper/project-tasks/pkg/logger"
	"github.com/benjasper/project-tasks/pkg/tasks"
	"github.com/benjasper/project-tasks/pkg/users"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"time"
)

func main() {
	apiVersion := "v1"
	var logging logger.Interface = logger.Logger{}
	fmt.Println("Server is starting up...")

	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://admin:123@localhost:27017/mongodb?authSource=admin&w=majority&readPreference=primary&retryWrites=true&ssl=false"))
	if err != nil {
		log.Fatal(err)
	}

	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		log.Panic(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Panic(err)
	}

	defer func() {
		err := client.Disconnect(ctx)
		if err != nil {
			logging.Fatal(err)
		}
	}()

	fmt.Println("Database connected")

	db := client.Database("test")

	userCollection := db.Collection("Users")
	taskCollection := db.Collection("Tasks")

	errorManager := communication.ErrorResponseManager{Logger: logging}

	var userService users.ServiceInterface = users.UserService{DB: userCollection, Logger: logging}
	userHandler := users.Handler{UserService: userService, Logger: logging, ErrorManager: &errorManager}

	var taskService tasks.TaskServiceInterface = tasks.TaskService{DB: taskCollection, Logger: logging}
	taskHandler := tasks.Handler{TaskService: taskService, Logger: logging, ErrorManager: &errorManager}

	r := mux.NewRouter()

	authMiddleWare := auth.AuthenticationMiddleware{ErrorManager: &errorManager}

	r.HandleFunc("/user", userHandler.HandleUserAdd).Methods(http.MethodPost)

	apiValidated := r.PathPrefix("/api/" + apiVersion).Subrouter()
	apiValidated.Use(authMiddleWare.Middleware)
	apiValidated.Path("/user/{id}").HandlerFunc(userHandler.HandleUserGet).Methods(http.MethodGet)
	apiValidated.Path("/task").HandlerFunc(taskHandler.TaskAdd).Methods(http.MethodPost)
	apiValidated.Path("/task/{taskID}").HandlerFunc(taskHandler.TaskUpdate).Methods(http.MethodPut)
	apiValidated.Path("/tasks").HandlerFunc(taskHandler.GetAllTasks).Methods(http.MethodGet)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	http.Handle("/", r)
	log.Panic(http.ListenAndServe(":80", r))
}
