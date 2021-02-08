package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
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

	userService := users.UserService{DB: userCollection, Logger: logging}
	userHandler := users.Handler{UserService: userService, Logger: logging, ErrorManager: &errorManager}
	calendarHandler := calendar.Handler{UserService: &userService, Logger: logging, ErrorManager: &errorManager}

	var taskService = tasks.TaskService{DB: taskCollection, Logger: logging}
	taskHandler := tasks.Handler{
		TaskService:  &taskService,
		Logger:       logging,
		ErrorManager: &errorManager,
		UserService:  &userService}

	r := mux.NewRouter()

	authMiddleWare := auth.AuthenticationMiddleware{ErrorManager: &errorManager}

	authAPI := r.PathPrefix("/" + apiVersion + "/auth/").Subrouter()
	authAPI.Path("/register").HandlerFunc(userHandler.UserRegister).Methods(http.MethodPost)
	authAPI.Path("/login").HandlerFunc(userHandler.UserLogin).Methods(http.MethodPost)
	authAPI.Path("/google").HandlerFunc(userHandler.GoogleCalendarAuthCallback).Methods(http.MethodGet)

	authenticatedAPI := r.PathPrefix("/" + apiVersion).Subrouter()
	authenticatedAPI.Use(authMiddleWare.Middleware)
	authenticatedAPI.Path("/user/{id}").HandlerFunc(userHandler.UserGet).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks").HandlerFunc(taskHandler.TaskAdd).Methods(http.MethodPost)
	authenticatedAPI.Path("/tasks/{taskID}").HandlerFunc(taskHandler.TaskUpdate).Methods(http.MethodPatch)
	authenticatedAPI.Path("/tasks/{taskID}").HandlerFunc(taskHandler.TaskDelete).Methods(http.MethodDelete)
	authenticatedAPI.Path("/tasks").HandlerFunc(taskHandler.GetAllTasks).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/{taskID}/workunits/{index}").HandlerFunc(taskHandler.WorkUnitUpdate).Methods(http.MethodPatch)
	authenticatedAPI.Path("/calendar/suggest").HandlerFunc(taskHandler.Suggest).Methods(http.MethodGet)
	authenticatedAPI.Path("/calendar/connect/google").
		HandlerFunc(userHandler.InitiateGoogleCalendarAuth).Methods(http.MethodPost)
	authenticatedAPI.Path("/calendars").HandlerFunc(calendarHandler.GetAllCalendars).Methods(http.MethodGet)
	authenticatedAPI.Path("/calendars").HandlerFunc(calendarHandler.PostCalendars).Methods(http.MethodPost)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	http.Handle("/", r)
	log.Panic(http.ListenAndServe(":80", r))
}
