package internal

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/spf13/viper"
)

type Config struct {
    User    	string   `mapstructure:"user"`
    Library		string   `mapstructure:"library"`
    VideoLib	string   `mapstructure:"videolibrary"`
    ImageExt	[]string `mapstructure:"image_extensions"`
    VideoExt	[]string `mapstructure:"video_extensions"`
    UseExifTool  bool
    UseHardlinks bool // Use hardlinks instead of copying files
}

func LoadConfig() (*Config, error) {
    configDir, err := os.UserConfigDir()
    if err != nil {
        return nil, fmt.Errorf("failed to find user config dir: %w", err)
    }

    viper.SetConfigName("anduril")
    viper.SetConfigType("toml")
    viper.AddConfigPath(filepath.Join(configDir, "anduril"))
    viper.AddConfigPath(filepath.Join(os.Getenv("HOME"), ".config", "anduril"))
    viper.AddConfigPath(".")

    // Set defaults:
    viper.SetDefault("user", "user")
    viper.SetDefault("library", filepath.Join(os.Getenv("HOME"), "anduril/images"))
    viper.SetDefault("videolibrary", filepath.Join(os.Getenv("HOME"), "anduril/videos"))
    viper.SetDefault("image_extensions", []string{
        ".jpg", ".jpeg", ".png", ".gif", ".heic", ".heif",
        ".tiff", ".tif", ".raw", ".cr2", ".nef", ".arw", ".raf", ".dng",
    })
    viper.SetDefault("video_extensions", []string{
        ".mp4", ".mov", ".avi", ".mkv", ".webm", ".flv", ".wmv", ".m4v",
    })

    if err := viper.ReadInConfig(); err != nil {
        // Config file not found; that's OK, just use defaults
        fmt.Printf("Config: No config file found, using defaults\n")
        fmt.Printf("  Searched: %s/anduril/anduril.toml\n", configDir)
        fmt.Printf("            %s/.config/anduril/anduril.toml\n", os.Getenv("HOME"))
        fmt.Printf("            ./anduril.toml\n")
    } else {
        fmt.Printf("Config: Loaded from %s\n", viper.ConfigFileUsed())
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }

    // Normalize extensions to lowercase to make matching case-insensitive
    for i, ext := range cfg.ImageExt {
        cfg.ImageExt[i] = strings.ToLower(ext)
    }
    for i, ext := range cfg.VideoExt {
        cfg.VideoExt[i] = strings.ToLower(ext)
    }

    return &cfg, nil
}
