package main

import (
	"cloud.google.com/go/profiler"
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/email"
	"github.com/timeliness-app/timeliness-backend/pkg/environment"
	"github.com/timeliness-app/timeliness-backend/pkg/locking"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println(err)
	}

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
		appEnv = environment.Dev
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "mongodb://admin:123@localhost:27017/mongodb?authSource=admin&w=majority&readPreference=primary&retryWrites=true&ssl=false"
	}

	database := os.Getenv("DATABASE")
	if database == "" {
		database = "test"
	}

	redisURL := os.Getenv("REDIS")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		redisPassword = ""
	}

	var logging logger.Interface = logger.Logger{}
	if appEnv == environment.Production {
		logging = logger.NewGoogleCloudLogger()
	}

	if appEnv == environment.Production {
		if err := profiler.Start(profiler.Config{}); err != nil {
			logging.Error("Could not start profiler", err)
		}
	}

	logging.Info("Server is starting up...")

	client, err := mongo.NewClient(options.Client().ApplyURI(databaseURL))
	if err != nil {
		log.Fatal(err)
	}

	var ctx, cancel = context.WithTimeout(context.Background(), 20*time.Second)
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

	redisClient := redis.NewClient(&redis.Options{
		Network:  "tcp",
		Addr:     redisURL,
		Password: redisPassword,
	})
	defer func(redisClient *redis.Client) {
		err := redisClient.Close()
		if err != nil {
			logging.Fatal(err)
		}
	}(redisClient)

	pong := redisClient.Ping(ctx)
	if pong.Err() != nil {
		logging.Fatal(pong.Err())
	}

	logging.Info("Redis connected")

	db := client.Database(database)

	userCollection := db.Collection("Users")
	taskCollection := db.Collection("Tasks")
	tagsCollection := db.Collection("Tags")

	secret := os.Getenv("SECRET")
	if secret == "" {
		secret = "local-secret"
	}

	locker := locking.NewLockerRedis(redisClient)

	responseManager := communication.ResponseManager{Logger: logging}
	userRepository := users.UserRepository{DB: userCollection, Logger: logging}

	calendarRepositoryManager, err := tasks.NewCalendarRepositoryManager(10, &userRepository, logging)
	if err != nil {
		logging.Fatal(err)
		return
	}

	// notificationController := notifications.NewNotificationController(logging, userRepository)

	var taskRepository = tasks.MongoDBTaskRepository{DB: taskCollection, Logger: logging}
	// taskRepository.Subscribe(&notificationController)

	planningService := tasks.NewPlanningController(&userRepository, &taskRepository, logging, locker, calendarRepositoryManager)

	emailService := email.NewSendInBlueService(os.Getenv("SENDINBLUE"))

	userHandler := users.Handler{UserRepository: &userRepository, Logger: logging, ResponseManager: &responseManager, Secret: secret, EmailService: emailService}
	calendarHandler := tasks.CalendarHandler{UserRepository: &userRepository, Logger: logging, ResponseManager: &responseManager,
		TaskRepository: &taskRepository, PlanningService: planningService, Locker: locker, CalendarRepositoryManager: calendarRepositoryManager}

	taskHandler := tasks.Handler{
		TaskRepository:  &taskRepository,
		Logger:          logging,
		Locker:          locker,
		ResponseManager: &responseManager,
		UserRepository:  &userRepository,
		PlanningService: planningService}

	tagRepository := tasks.TagRepository{Logger: logging, DB: tagsCollection}
	tagHandler := tasks.TagHandler{
		Logger: logging, TagRepository: tagRepository, ResponseManager: &responseManager, UserRepository: &userRepository,
		TaskRepository: &taskRepository,
	}

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

	// TODO: remove when launching
	// unauthenticatedAPI.Path("/auth/register").HandlerFunc(userHandler.UserRegister).Methods(http.MethodPost)
	unauthenticatedAPI.Path("/auth/register/verify").HandlerFunc(userHandler.VerifyRegistrationGet).Methods(http.MethodGet)
	unauthenticatedAPI.Path("/auth/refresh").HandlerFunc(userHandler.UserRefresh).Methods(http.MethodPost)
	unauthenticatedAPI.Path("/auth/login").HandlerFunc(userHandler.UserLogin).Methods(http.MethodPost)
	unauthenticatedAPI.Path("/auth/google").HandlerFunc(calendarHandler.GoogleCalendarAuthCallback).Methods(http.MethodGet)

	unauthenticatedAPI.Path("/calendar/google/notifications").
		HandlerFunc(calendarHandler.GoogleCalendarNotification).Methods(http.MethodPost)
	unauthenticatedAPI.Path("/calendar/google/notifications/renew").
		HandlerFunc(calendarHandler.GoogleCalendarSyncRenewal).Methods(http.MethodPost)

	unauthenticatedAPI.Path("/newsletter").
		HandlerFunc(userHandler.RegisterForNewsletter).Methods(http.MethodPost)

	authenticatedAPI := r.PathPrefix("/" + apiVersion).Subrouter()
	authenticatedAPI.Use(authMiddleWare.Middleware)
	authenticatedAPI.Path("/user").HandlerFunc(userHandler.UserGet).Methods(http.MethodGet)
	authenticatedAPI.Path("/user/settings").HandlerFunc(userHandler.UserSettingsPatch).Methods(http.MethodPatch)
	authenticatedAPI.Path("/user/device").HandlerFunc(userHandler.UserAddDevice).Methods(http.MethodPost)
	authenticatedAPI.Path("/user/device/{deviceToken}").HandlerFunc(userHandler.UserRemoveDevice).Methods(http.MethodDelete)

	authenticatedAPI.Path("/tasks").HandlerFunc(taskHandler.TaskAdd).Methods(http.MethodPost)
	authenticatedAPI.Path("/tasks").HandlerFunc(taskHandler.GetAllTasks).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/between").HandlerFunc(taskHandler.GetTaskBetween).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/workunits").HandlerFunc(taskHandler.GetAllTasksByWorkUnits).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/workunits/between").HandlerFunc(taskHandler.GetWorkUnitsBetween).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/agenda").HandlerFunc(taskHandler.GetTasksByAgenda).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/{taskID}").HandlerFunc(taskHandler.TaskGet).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/{taskID}").HandlerFunc(taskHandler.TaskUpdate).Methods(http.MethodPatch)
	authenticatedAPI.Path("/tasks/{taskID}").HandlerFunc(taskHandler.TaskDelete).Methods(http.MethodDelete)
	authenticatedAPI.Path("/tasks/{taskID}/workunits/{index}").HandlerFunc(taskHandler.WorkUnitUpdate).Methods(http.MethodPatch)
	authenticatedAPI.Path("/tasks/{taskID}/workunits/{index}/done").HandlerFunc(taskHandler.MarkWorkUnitAsDone).Methods(http.MethodPatch)
	authenticatedAPI.Path("/tasks/{taskID}/workunits/{index}/reschedule").HandlerFunc(taskHandler.RescheduleWorkUnitGet).Methods(http.MethodGet)
	authenticatedAPI.Path("/tasks/{taskID}/workunits/{index}/reschedule").HandlerFunc(taskHandler.RescheduleWorkUnitPost).Methods(http.MethodPost)

	authenticatedAPI.Path("/tags").HandlerFunc(tagHandler.TagAdd).Methods(http.MethodPost)
	authenticatedAPI.Path("/tags").HandlerFunc(tagHandler.GetAllTags).Methods(http.MethodGet)
	authenticatedAPI.Path("/tags/{tagID}").HandlerFunc(tagHandler.TagUpdate).Methods(http.MethodPatch)
	authenticatedAPI.Path("/tags/{tagID}").HandlerFunc(tagHandler.TagDelete).Methods(http.MethodDelete)

	authenticatedAPI.Path("/connections/google").HandlerFunc(calendarHandler.InitiateGoogleCalendarAuth).Methods(http.MethodPost)
	authenticatedAPI.Path("/connections/{connectionID}/google").HandlerFunc(calendarHandler.InitiateGoogleCalendarAuth).Methods(http.MethodPost)
	authenticatedAPI.Path("/connections/{connectionID}/google").HandlerFunc(calendarHandler.DeleteGoogleConnection).Methods(http.MethodDelete)
	authenticatedAPI.Path("/connections/{connectionID}/calendars").HandlerFunc(calendarHandler.GetCalendarsFromConnection).Methods(http.MethodGet)
	authenticatedAPI.Path("/connections/{connectionID}/calendars").HandlerFunc(calendarHandler.PatchCalendars).Methods(http.MethodPut)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if accessControl == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if strings.HasSuffix(origin, accessControl) || strings.Contains(origin, "http://localhost:") {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			w.Header().Add("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	http.Handle("/", r)
	server := http.Server{Addr: ":" + port, Handler: r}

	go func() {
		if err = server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				logging.Info("Server was shutdown")
				return
			}
			logging.Fatal(err)
		}
	}()

	logging.Info("Server started on port " + port)

	// Setting up signal capturing
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Waiting for SIGINT (kill -2)
	<-stop

	logging.Info("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logging.Error("Error shutting down server: ", err)
	}
}
