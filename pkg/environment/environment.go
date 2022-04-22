package environment

import (
	"github.com/joho/godotenv"
	"github.com/mitchellh/mapstructure"
)

// Production defines the prod environment
const Production = "prod"

// Staging defines the staging environment
const Staging = "staging"

// Dev defines the dev environment
const Dev = "dev"

// Environment defines the environment
type Environment struct {
	Environment             string `mapstructure:"APP_ENV"`
	Cors                    string `mapstructure:"CORS"`
	Secret                  string `mapstructure:"SECRET"`
	SchedulerSecret         string `mapstructure:"SCHEDULER_SECRET"`
	Database                string `mapstructure:"DATABASE"`
	DatabaseURL             string `mapstructure:"DATABASE_URL"`
	Redis                   string `mapstructure:"REDIS"`
	RedisPassword           string `mapstructure:"REDIS_PASSWORD"`
	Sendinblue              string `mapstructure:"SENDINBLUE"`
	StripeLive              string `mapstructure:"STRIPE_LIVE"`
	StripeTest              string `mapstructure:"STRIPE_TEST"`
	StripeWebhookSecret     string `mapstructure:"STRIPE_WEBHOOK_SECRET"`
	StripeWebhookSecretTest string `mapstructure:"STRIPE_WEBHOOK_SECRET_TEST"`
	BaseURL                 string `mapstructure:"BASE_URL"`
	FrontendBaseURL         string `mapstructure:"FRONTEND_BASE_URL"`
}

// Global holds the global environment variables
var Global Environment

// Initialize loads the environment variables into Global
func Initialize() {
	data, err := godotenv.Read(".env")
	if err != nil {
		panic(err)
	}

	err = mapstructure.Decode(data, &Global)
	if err != nil {
		panic(err)
	}
}
