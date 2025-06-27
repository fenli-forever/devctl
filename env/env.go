package env

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jd/devctl/config"
	"github.com/jd/devctl/logger"
	"github.com/jd/devctl/ssh"
)

type EnvManager struct {
	Config *config.Config
	log    *logger.Logger
}

func NewEnvManager(cfg *config.Config, log *logger.Logger) *EnvManager {
	return &EnvManager{Config: cfg, log: log}
}

func (em *EnvManager) ListEnvironments() []config.Environment {
	return em.Config.Envs
}

func (em *EnvManager) AddEnvironment(env config.Environment) error {
	em.log.Info("Adding new environment: %s", env.ID)
	for _, e := range em.Config.Envs {
		if e.ID == env.ID {
			em.log.Error("Environment with ID %s already exists", env.ID)
			return fmt.Errorf("environment with ID %s already exists", env.ID)
		}
	}

	// Download kubeconfig file
	kubeconfigPath, err := em.downloadKubeconfig(env)
	if err != nil {
		em.log.Error("Failed to download kubeconfig: %v", err)
		return err
	}

	env.Kubeconfig = kubeconfigPath
	env.CreateTime = time.Now().Format("2006-01-02 15:04:05")
	env.UpdateTime = env.CreateTime
	em.Config.Envs = append(em.Config.Envs, env)
	err = config.SaveConfig(em.Config, em.log)
	if err != nil {
		em.log.Error("Failed to save config after adding environment: %v", err)
		return err
	}
	em.log.Info("Environment %s added successfully", env.ID)
	return nil
}

func (em *EnvManager) downloadKubeconfig(env config.Environment) (string, error) {
	sshClient := ssh.NewSSHClient(env.IP, env.User, env.Password)

	remoteFile := "/root/.kube/config"
	localDir := filepath.Join(os.Getenv("HOME"), ".devctl", "kubeconfigs", env.ID)
	localFile := filepath.Join(localDir, "config")

	err := os.MkdirAll(localDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create local directory: %v", err)
	}

	err = sshClient.DownloadFile(remoteFile, localFile)
	if err != nil {
		return "", fmt.Errorf("failed to download kubeconfig: %v", err)
	}

	em.log.Info("Kubeconfig downloaded successfully for environment %s", env.ID)
	return localFile, nil
}

func (em *EnvManager) UpdateEnvironment(env config.Environment) error {
	em.log.Info("Updating environment: %s", env.ID)
	for i, e := range em.Config.Envs {
		if e.ID == env.ID {
			env.UpdateTime = time.Now().Format("2006-01-02 15:04:05")
			em.Config.Envs[i] = env
			err := config.SaveConfig(em.Config, em.log)
			if err != nil {
				em.log.Error("Failed to save config after updating environment: %v", err)
				return err
			}
			em.log.Info("Environment %s updated successfully", env.ID)
			return nil
		}
	}
	em.log.Error("Environment with ID %s not found", env.ID)
	return fmt.Errorf("environment with ID %s not found", env.ID)
}

func (em *EnvManager) DeleteEnvironment(id string) error {
	em.log.Info("Deleting environment: %s", id)
	for i, e := range em.Config.Envs {
		if e.ID == id {
			em.Config.Envs = append(em.Config.Envs[:i], em.Config.Envs[i+1:]...)
			err := config.SaveConfig(em.Config, em.log)
			if err != nil {
				em.log.Error("Failed to save config after deleting environment: %v", err)
				return err
			}
			em.log.Info("Environment %s deleted successfully", id)
			return nil
		}
	}
	em.log.Error("Environment with ID %s not found", id)
	return fmt.Errorf("environment with ID %s not found", id)
}

func (em *EnvManager) GetEnvironment(id string) (config.Environment, error) {
	em.log.Info("Getting environment: %s", id)
	for _, e := range em.Config.Envs {
		if e.ID == id {
			return e, nil
		}
	}
	em.log.Error("Environment with ID %s not found", id)
	return config.Environment{}, fmt.Errorf("environment with ID %s not found", id)
}
