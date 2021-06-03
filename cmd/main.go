package main

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	apiVersion := "v1"
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	accessControl := os.Getenv("CORS")
	if accessControl == "" {
		accessControl = "*"
	}

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "dev"
	}

	databaseURL := os.Getenv("DATABASE")
	if databaseURL == "" {
		databaseURL = "mongodb://admin:123@localhost:27017/mongodb?authSource=admin&w=majority&readPreference=primary&retryWrites=true&ssl=false"
	}

	var logging logger.Interface = logger.Logger{}
	if appEnv == "prod" {
		logging = logger.NewGoogleCloudLogger()
	}

	logging.Info("Server is starting up...")

	client, err := mongo.NewClient(options.Client().ApplyURI(databaseURL))
	if err != nil {
		log.Fatal(err)
	}

	var ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		logging.Fatal(err)
		log.Panic(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		logging.Fatal(err)
		log.Panic(err)
	}

	defer func() {
		err := client.Disconnect(ctx)
		if err != nil {
			logging.Fatal(err)
		}
	}()

	logging.Info("Database connected")

	db := client.Database("test")

	userCollection := db.Collection("Users")
	taskCollection := db.Collection("Tasks")

	secret := os.Getenv("SECRET")
	if secret == "" {
		secret = "local-secret"
	}

	responseManager := communication.ResponseManager{Logger: logging}

	var taskService = tasks.TaskService{DB: taskCollection, Logger: logging}

	userService := users.UserService{DB: userCollection, Logger: logging}
	userHandler := users.Handler{UserService: userService, Logger: logging, ResponseManager: &responseManager, Secret: secret}
	calendarHandler := tasks.CalendarHandler{UserService: &userService, Logger: logging, ResponseManager: &responseManager,
		TaskService: &taskService}

	taskHandler := tasks.Handler{
		TaskService:     &taskService,
		Logger:          logging,
		ResponseManager: &responseManager,
		UserService:     &userService}

	r := mux.NewRouter()

	authMiddleWare := auth.AuthenticationMiddleware{ErrorManager: &responseManager, Secret: secret}

	r.Methods(http.MethodOptions).HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, PATCH ,OPTIONS, DELETE")
			w.Header().Set("Access-Control-Max-Age", "804800")
		})

	r.Path("/").HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("Welcome to the Timeliness API 🚀"))
	})

	unauthenticatedAPI := r.PathPrefix("/" + apiVersion).Subrouter()
	unauthenticatedAPI.Path("/auth/register").HandlerFunc(userHandler.UserRegister).Methods(http.MethodPost)
	unauthenticatedAPI.Path("/auth/refresh").HandlerFunc(userHandler.UserRefresh).Methods(http.MethodPost)
	unauthenticatedAPI.Path("/auth/login").HandlerFunc(userHandler.UserLogin).Methods(http.MethodPost)
	unauthenticatedAPI.Path("/auth/google").HandlerFunc(userHandler.GoogleCalendarAuthCallback).Methods(http.MethodGet)
	unauthenticatedAPI.Path("/calendar/google/notifications").
		HandlerFunc(calendarHandler.GoogleCalendarNotification).Methods(http.MethodPost)
	unauthenticatedAPI.Path("/calendar/google/notifications/renew").
		HandlerFunc(calendarHandler.GoogleCalendarSyncRenewal).Methods(http.MethodPost)

	authenticatedAPI := r.PathPrefix("/" + apiVersion).Subrouter()
	authenticatedAPI.Use(authMiddleWare.Middleware)
	authenticatedAPI.Path("/user/{id}").HandlerFunc(userHandler.UserGet).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks").HandlerFunc(taskHandler.TaskAdd).Methods(http.MethodPost)
	authenticatedAPI.Path("/tasks").HandlerFunc(taskHandler.GetAllTasks).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/workunits").HandlerFunc(taskHandler.GetAllTasksByWorkUnits).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/{taskID}").HandlerFunc(taskHandler.TaskGet).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/{taskID}").HandlerFunc(taskHandler.TaskUpdate).Methods(http.MethodPatch)
	authenticatedAPI.Path("/tasks/{taskID}").HandlerFunc(taskHandler.TaskDelete).Methods(http.MethodDelete)
	authenticatedAPI.Path("/tasks/{taskID}/workunits/{index}").HandlerFunc(taskHandler.WorkUnitUpdate).Methods(http.MethodPatch)
	authenticatedAPI.Path("/tasks/{taskID}/workunits/{index}/reschedule").HandlerFunc(taskHandler.RescheduleWorkUnit).Methods(http.MethodPost)
	authenticatedAPI.Path("/calendar/google/connect").
		HandlerFunc(userHandler.InitiateGoogleCalendarAuth).Methods(http.MethodPost)
	authenticatedAPI.Path("/calendars").HandlerFunc(calendarHandler.GetAllCalendars).Methods(http.MethodGet)
	authenticatedAPI.Path("/calendars").HandlerFunc(calendarHandler.PostCalendars).Methods(http.MethodPost)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", accessControl)
			w.Header().Add("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	http.Handle("/", r)
	logging.Fatal(http.ListenAndServe(":"+port, r))
}
