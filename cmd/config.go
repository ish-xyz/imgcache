package cmd

import (
	"time"

	"github.com/ish-xyz/registry-cache/pkg/proxy"
	"github.com/go-playground/validator"
	"github.com/inhies/go-bytesize"
	"github.com/spf13/viper"
)

type Config struct {
	DataPath string `mapstructure:"dataPath" validate:"required"`
	Server   struct {
		Address         string              `mapstructure:"address" validate:"required" yaml:"address"`
		UpstreamTimeout time.Duration       `mapstructure:"upstreamTimeout" validate:"valid-time,required" yaml:"upstreamTimeout"`
		Timeout         time.Duration       `mapstructure:"upstreamTimeout" validate:"valid-time,required" yaml:"timeout"`
		Workers         int                 `mapstructure:"workers" validate:"valid-workers-number,required" yaml:"workers"`
		UpstreamRules   []map[string]string `mapstructure:"upstreamRules" validate:"valid-upstream-rules,required" yaml:"upstreamRules"`
		DefaultBackend  struct {
			Host   string `mapstructure:"host" validate:"required" yaml:"host"`
			Scheme string `mapstructure:"scheme" validate:"required" yaml:"scheme"`
		} `mapstructure:"defaultBackend" validate:"required" yaml:"defaultBackend"`
		TLS struct {
			CAPath   string `mapstructure:"caPath" validate:"required" yaml:"CAPath"`
			CertPath string `mapstructure:"certPath" validate:"required" yaml:"certPath"`
			KeyPath  string `mapstructure:"keyPath" validate:"required" yaml:"keyPath"`
		} `mapstructure:"tls" validate:"required" yaml:"tls"`
	}

	Metrics struct {
		Address string `mapstructure:"address" validate:"required" yaml:"address"`
	} `mapstructure:"metrics" validate:"required" yaml:"metrics"`

	GC struct {
		Interval time.Duration `mapstructure:"interval" validate:"required,valid-min-time" yaml:"interval"`

		Disk struct {
			MaxSize string `mapstructure:"maxSize" validate:"required,valid-bsize" yaml:"maxSize"`
		} `mapstructure:"disk" validate:"required" yaml:"disk"`

		Layers struct {
			CheckSHA  bool          `mapstructure:"checkSHA"`
			MaxAge    time.Duration `mapstructure:"maxAge" validate:"valid-min-time,required" yaml:"maxAge"`
			MaxUnused time.Duration `mapstructure:"maxUnused" validate:"valid-min-time,required" yaml:"maxUnused"`
		} `mapstructure:"layers" validate:"required" yaml:"layers"`
		Manifests struct {
			MaxAge    time.Duration `mapstructure:"maxAge" validate:"valid-min-time,required" yaml:"maxAge"`
			MaxUnused time.Duration ` mapstructure:"maxUnused" validate:"valid-min-time,required" yaml:"maxUnused"`
		} `mapstructure:"manifests" validate:"required" yaml:"manifests"`
	} `mapstructure:"gc" validate:"required" yaml:"gc"`
}

func LoadAndValidateConfig(configFile string) (*Config, error) {

	var c Config

	viper.SetConfigType("yaml")
	viper.SetConfigFile(configFile)
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	err = viper.Unmarshal(&c)
	if err != nil {
		return nil, err
	}

	val := NewValidator()

	if err := val.Struct(c); err != nil {
		return nil, err
	}

	return &c, nil
}

func NewValidator() *validator.Validate {

	validate := validator.New()

	validate.RegisterValidation("valid-min-time", ValidateMinTime)
	validate.RegisterValidation("valid-time", ValidateTime)
	validate.RegisterValidation("valid-bsize", ValidateBSize)
	validate.RegisterValidation("valid-upstream-rules", ValidateUpstreamRules)
	validate.RegisterValidation("valid-workers-number", ValidateMinWorkers)

	return validate
}

func getUpstreamRules(rules []map[string]string) ([]*proxy.UpstreamRule, error) {
	var urules = make([]*proxy.UpstreamRule, 0)
	for _, r := range rules {

		u, err := proxy.NewUpstreamRule(r["host"], r["scheme"], r["regex"])
		if err != nil {
			return nil, err
		}
		urules = append(urules, u)
	}
	return urules, nil
}

// Validators

func ValidateTime(fl validator.FieldLevel) bool {

	minValue := time.Duration(time.Second * 1)
	value, ok := fl.Field().Interface().(time.Duration)
	if !ok {
		return false
	}
	return value >= minValue
}

func ValidateMinTime(fl validator.FieldLevel) bool {

	minValue := time.Duration(time.Second * 60)
	value, ok := fl.Field().Interface().(time.Duration)
	if !ok {
		return false
	}
	return value >= minValue
}

func ValidateUpstreamRules(fl validator.FieldLevel) bool {

	rules, ok := fl.Field().Interface().([]map[string]string)
	if !ok {
		return false
	}
	for _, r := range rules {
		if _, ok := r["host"]; !ok {
			return false
		}
		if _, ok := r["scheme"]; !ok {
			return false
		}
		if _, ok := r["regex"]; !ok {
			return false
		}
	}

	return true
}

func ValidateBSize(fl validator.FieldLevel) bool {
	sizeStr, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}

	_, err := bytesize.Parse(sizeStr)
	return err == nil
}

func ValidateMinWorkers(fl validator.FieldLevel) bool {
	wn, ok := fl.Field().Interface().(int)
	if !ok {
		return false
	}

	return wn >= 1
}
