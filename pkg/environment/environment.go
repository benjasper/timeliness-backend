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

type Environment struct {
	Environment             string `mapstructure:"APP_ENV"`
	Cors                    string `mapstructure:"CORS"`
	Secret                  string `mapstructure:"SECRET"`
	SchedulerSecret         string `mapstructure:"SCHEDULER_SECRET"`
	Port                    string `mapstructure:"PORT"`
	Database                string `mapstructure:"DATABASE"`
	DatabaseUrl             string `mapstructure:"DATABASE_URL"`
	Redis                   string `mapstructure:"REDIS"`
	RedisPassword           string `mapstructure:"REDIS_PASSWORD"`
	Sendinblue              string `mapstructure:"SENDINBLUE"`
	StripeLive              string `mapstructure:"STRIPE_LIVE"`
	StripeTest              string `mapstructure:"STRIPE_TEST"`
	StripeWebhookSecret     string `mapstructure:"STRIPE_WEBHOOK_SECRET"`
	StripeWebhookSecretTest string `mapstructure:"STRIPE_WEBHOOK_SECRET_TEST"`
	BaseUrl                 string `mapstructure:"BASE_URL"`
	FrontendBaseUrl         string `mapstructure:"FRONTEND_BASE_URL"`
}

var Global Environment

func Initialize() {
	data, err := godotenv.Read(".env")
	if err != nil {
		panic(err)
		return
	}

	err = mapstructure.Decode(data, &Global)
	if err != nil {
		panic(err)
		return
	}
}
