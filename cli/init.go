package cli

import (
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	cfgName string
)

func initCobra() {

	viper.SetEnvPrefix("minibridge")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	dataFolder, err := xdg.DataFile("minibridge")
	if err != nil {
		slog.Error("failed to retrieve xdg data folder: %w", err)
		os.Exit(1)
	}

	configFolder, err := xdg.ConfigFile("minibridge")
	if err != nil {
		slog.Error("failed to retrieve xdg data folder: %w", err)
		os.Exit(1)
	}

	slog.Debug("Folders configured", "config", configFolder, "data", dataFolder)

	if cfgFile == "" {
		cfgFile = os.Getenv("MINIBRIDGE_CONFIG")
	}

	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			slog.Error("Config file does not exist", err)
			os.Exit(1)
		}

		viper.SetConfigType("yaml")
		viper.SetConfigFile(cfgFile)

		if err = viper.ReadInConfig(); err != nil {
			slog.Error("Unable to read config",
				"path", cfgFile,
				err,
			)
			os.Exit(1)
		}

		slog.Debug("Using config file", "path", cfgFile)
		return
	}

	viper.AddConfigPath(configFolder)
	viper.AddConfigPath("/usr/local/etc/minibridge")
	viper.AddConfigPath("/etc/minibridge")

	if cfgName == "" {
		cfgName = os.Getenv("MINIBRIDGE_CONFIG_NAME")
	}

	if cfgName == "" {
		cfgName = "default"
	}

	viper.SetConfigName(cfgName)

	if err = viper.ReadInConfig(); err != nil {
		if !errors.As(err, &viper.ConfigFileNotFoundError{}) {
			slog.Error("Unable to read config", err)
			os.Exit(1)
		}
	}

	slog.Debug("Using config name", "name", cfgName)
}
