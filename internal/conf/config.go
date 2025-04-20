package conf

import (
	"reflect"
	"strings"

	"github.com/creasty/defaults"
	"github.com/getnimbus/ultrago/u_logger"
	"github.com/spf13/viper"
)

var Config config

func init() {
	if err := defaults.Set(&Config); err != nil {
		panic(err)
	}
	bindEnvs(Config)
}

// Config stores all configuration of the application.
// The values are read by viper from a config file or environment variable.
type config struct {
	// constant
	Env   string `mapstructure:"ENV" default:"dev"`
	Debug string `mapstructure:"DEBUG" default:"no"`

	// db
	Migration string `mapstructure:"MIGRATION" default:"no"`
	GormDsn   string `mapstructure:"GORM_DSN" default:"-"`

	// aws
	AwsEndpoint        string `mapstructure:"AWS_ENDPOINT" default:"-"`
	AwsRegion          string `mapstructure:"AWS_REGION" default:"-"`
	AwsAccessKeyId     string `mapstructure:"AWS_ACCESS_KEY_ID" default:"-"`
	AwsSecretAccessKey string `mapstructure:"AWS_SECRET_ACCESS_KEY" default:"-"`
	AwsBucket          string `mapstructure:"AWS_BUCKET" default:"-indexer"`
	AthenaQueryResult  string `mapstructure:"ATHENA_QUERY_RESULT" default:"s3://nimbus-result/athena-query/"`

	// kafka
	KafkaBrokers       string `mapstructure:"KAFKA_BROKERS" default:"localhost:9092"`
	KafkaConsumerGroup string `mapstructure:"KAFKA_CONSUMER_GROUP" default:"alephium-consumer"`
	KafkaUsername      string `mapstructure:"KAFKA_USERNAME" default:"-"`
	KafkaPassword      string `mapstructure:"KAFKA_PASSWORD" default:"-"`
	AlphBlocksTopic    string `mapstructure:"ALPH_BLOCKS_TOPIC" default:"alph-blocks"`
	AlphTxsTopic       string `mapstructure:"ALPH_TXS_TOPIC" default:"alph-txs"`
	AlphEventsTopic    string `mapstructure:"ALPH_EVENTS_TOPIC" default:"alph-events"`

	// redis
	RedisAddress  string `mapstructure:"REDIS_ADDRESS" default:"localhost:6379"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD" default:"-"`
	RedisDB       int    `mapstructure:"REDIS_DB" default:"0"`

	// external service
	DiscordWebhook string `mapstructure:"DISCORD_WEBHOOK" default:"-"`
}

func (c *config) IsLocal() bool {
	return !c.IsDev() && !c.IsProd()
}

func (c *config) IsDev() bool {
	return strings.ToLower(c.Env) == "dev"
}

func (c *config) IsProd() bool {
	return strings.ToLower(c.Env) == "prod"
}

func (c *config) IsDebug() bool {
	return strings.ToLower(c.Debug) == "yes"
}

func (c *config) IsMigration() bool {
	return strings.ToLower(c.Migration) == "yes"
}

// LoadConfig read configuration for both file and system environment
func LoadConfig(path string) error {
	logger := u_logger.NewLogger()

	// Read from .env file
	viper.AddConfigPath(path)
	viper.SetConfigFile(".env")

	// priority load os ENV before .env file
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warnf("config file not found: %v", err)
		} else {
			logger.Warnf("config file is invalid: %v", err)
		}
	}

	if err := viper.Unmarshal(&Config); err != nil {
		return err
	}

	return nil
}

func bindEnvs(iface interface{}, parts ...string) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		t := ift.Field(i)
		tv, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			continue
		}
		switch v.Kind() {
		case reflect.Struct:
			bindEnvs(v.Interface(), append(parts, tv)...)
		default:
			viper.BindEnv(strings.Join(append(parts, tv), "."))
		}
	}
}
