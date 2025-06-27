package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/jd/devctl/logger"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Envs []Environment `yaml:"envs"`
}

type Environment struct {
	ID         string `yaml:"id"`
	Name       string `yaml:"name"`
	CreateTime string `yaml:"createTime"`
	UpdateTime string `yaml:"updateTime"`
	IP         string `yaml:"ip"`
	User       string `yaml:"user"`
	Password   string `yaml:"password"`
	Kubeconfig string `yaml:"kubeconfig"`
}

func LoadConfig(log *logger.Logger) (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Error("Failed to get user home directory: %v", err)
		return nil, err
	}

	configPath := filepath.Join(home, ".devctl", "config.yaml")
	log.Info("Loading config from: %s", configPath)

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Error("Failed to read config file: %v", err)
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Error("Failed to unmarshal config data: %v", err)
		return nil, err
	}

	log.Info("Config loaded successfully")
	return &config, nil
}

func SaveConfig(config *Config, log *logger.Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Error("Failed to get user home directory: %v", err)
		return err
	}

	configPath := filepath.Join(home, ".devctl", "config.yaml")
	log.Info("Saving config to: %s", configPath)

	// Backup the existing config file before saving
	if err := BackupConfig(configPath, log); err != nil {
		log.Error("Failed to backup config: %v", err)
		return fmt.Errorf("failed to backup config: %v", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		log.Error("Failed to marshal config data: %v", err)
		return err
	}

	err = os.MkdirAll(filepath.Dir(configPath), 0755)
	if err != nil {
		log.Error("Failed to create config directory: %v", err)
		return err
	}

	err = ioutil.WriteFile(configPath, data, 0644)
	if err != nil {
		log.Error("Failed to write config file: %v", err)
		return err
	}

	log.Info("Config saved successfully")
	return nil
}

func BackupConfig(configPath string, log *logger.Logger) error {
	// Check if the config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Info("Config file doesn't exist, no need to backup")
		return nil
	}

	// Read the existing config file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Error("Failed to read config file for backup: %v", err)
		return err
	}

	// Create a backup filename with timestamp
	backupDir := filepath.Join(filepath.Dir(configPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Error("Failed to create backup directory: %v", err)
		return err
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("config_%s.yaml", timestamp))

	// Write the backup file
	err = ioutil.WriteFile(backupPath, data, 0644)
	if err != nil {
		log.Error("Failed to write backup file: %v", err)
		return err
	}

	log.Info("Config backup created: %s", backupPath)
	return nil
}
