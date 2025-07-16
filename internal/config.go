package internal

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/viper"
)

type Config struct {
    User    	string   `mapstructure:"user"`
    Library		string   `mapstructure:"library"`
    VideoLib	string   `mapstructure:"videolibrary"`
    ImageExt	[]string `mapstructure:"image_extensions"`
    VideoExt	[]string `mapstructure:"video_extensions"`
}

func LoadConfig() (*Config, error) {
    configDir, err := os.UserConfigDir()
    if err != nil {
        return nil, fmt.Errorf("failed to find user config dir: %w", err)
    }

    viper.SetConfigName("anduril")
    viper.SetConfigType("toml")
    viper.AddConfigPath(filepath.Join(configDir, "anduril"))

    // Set defaults:
    viper.SetDefault("user", "user")
    viper.SetDefault("library", filepath.Join(os.Getenv("HOME"), "anduril/images"))
    viper.SetDefault("videolibrary", filepath.Join(os.Getenv("HOME"), "anduril/videos"))
    viper.SetDefault("image_extensions", []string{".jpg", ".jpeg", ".png", ".gif", ".heic"})
    viper.SetDefault("video_extensions", []string{".mp4", ".mov", ".avi", ".mkv"})

    if err := viper.ReadInConfig(); err != nil {
        // Config file not found; that's OK, just use defaults
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }

    return &cfg, nil
}
