package notifications

import (
	"context"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"google.golang.org/api/option"
	"os"
)

// NotificationController can send Messages to Google Cloud Messaging
type NotificationController struct {
	Logger         logger.Interface
	Client         messaging.Client
	UserRepository users.UserRepositoryInterface
}

// NewNotificationController construct a NotificationController
func NewNotificationController(logger logger.Interface, userRepository users.UserRepositoryInterface) NotificationController {
	ctrl := NotificationController{}
	ctx := context.Background()

	key := os.Getenv("FIREBASE")
	projectID := os.Getenv("GCP_PROJECT_ID")

	opt := option.WithAPIKey(key)
	config := &firebase.Config{ProjectID: projectID}
	app, err := firebase.NewApp(ctx, config, opt)
	if err != nil {
		logger.Fatal(err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	ctrl.Client = *client
	ctrl.Logger = logger
	ctrl.UserRepository = userRepository

	return ctrl
}

// OnNotify gets called when a task changes
func (n *NotificationController) OnNotify(task *tasks.Task) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	user, err := n.UserRepository.FindByID(ctx, task.UserID.Hex())
	if err != nil {
		n.Logger.Error("Could not find user", err)
		return
	}

	if len(user.DeviceTokens) == 0 {
		return
	}

	var tokens []string
	for _, token := range user.DeviceTokens {
		tokens = append(tokens, token.Token)
	}

	message := &messaging.MulticastMessage{
		Data: map[string]string{
			"collapse_key": "sync",
		},
		Tokens: tokens,
	}

	_, err = n.Client.SendMulticast(ctx, message)
	if err != nil {
		n.Logger.Error("Could not send messaging request", err)
	}
}
