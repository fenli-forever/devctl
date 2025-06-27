package main

import (
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
	logFile := filepath.Join(homeDir, ".devctl", "devctl.log")
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
