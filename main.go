package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jd/devctl/config"
	"github.com/jd/devctl/logger"
	"github.com/jd/devctl/ui"
)

var log *logger.Logger

func main() {
	// Set up logging
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic("Failed to get user home directory")
	}
	logDir := filepath.Join(homeDir, ".devctl")
	logFile := filepath.Join(logDir, "devctl.log")
	// 确保目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create log directory: %v", err))
	}
	// 确保文件存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		if file, err := os.Create(logFile); err != nil {
			panic(fmt.Sprintf("Failed to create log file: %v", err))
		} else {
			file.Close()
			os.Chmod(logFile, 0644)
		}
	}
	log, err = logger.NewLogger(logger.INFO, logFile)
	if err != nil {
		panic("Failed to initialize logger")
	}
	defer log.Close()

	log.Info("Starting devctl application")

	// Load configuration
	cfg, err := config.LoadConfig(log)
	if err != nil {
		log.Error("Error loading config: %v", err)
		log.Info("Using empty config")
		cfg = &config.Config{} // Use empty config if loading fails
	}

	// Initialize UI
	ui := ui.NewUI(cfg, log)
	log.Info("Initializing UI")

	// Run UI
	if err := ui.Run(); err != nil {
		log.Error("Error running UI: %v", err)
		os.Exit(1)
	}

	log.Info("devctl application exiting")
}
