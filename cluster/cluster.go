package cluster

import (
	"context"
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jd/devctl/config"
	"github.com/jd/devctl/logger"
	"github.com/jd/devctl/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ClusterManager struct {
	EnvID  string
	Config *config.Config
	log    *logger.Logger
}

func NewClusterManager(envID string, cfg *config.Config, log *logger.Logger) *ClusterManager {
	return &ClusterManager{
		EnvID:  envID,
		Config: cfg,
		log:    log,
	}
}

type ClusterInfo struct {
	ID         string
	Name       string
	OS         string
	ARCH       string
	Region     string
	Kubeconfig string
	ApiServer  string
	Cri        string
	Version    string
	Status     string
}

type NodeInfo struct {
	Name string
	IP   string
}

func (cm *ClusterManager) ListClusterNodes(clusterName string) ([]NodeInfo, error) {
	cm.log.Info("Listing nodes for cluster: %s", clusterName)

	kubeconfigPath, err := cm.GetKubeconfig(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig for cluster %s: %v", clusterName, err)
	}

	clientset, err := cm.getClientset(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get clientset for cluster %s: %v", clusterName, err)
	}

	nodeList, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes for cluster %s: %v", clusterName, err)
	}

	var nodes []NodeInfo
	for _, node := range nodeList.Items {
		var internalIP string
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				internalIP = addr.Address
				break
			}
		}
		if internalIP != "" {
			nodes = append(nodes, NodeInfo{
				Name: node.Name,
				IP:   internalIP,
			})
		}
	}

	cm.log.Info("Found %d nodes for cluster %s", len(nodes), clusterName)
	return nodes, nil
}

func (cm *ClusterManager) ListClusters() ([]ClusterInfo, error) {
	cm.log.Info("Listing clusters")

	env, err := cm.getEnvironment()
	if err != nil {
		cm.log.Error("Failed to get environment, err: %v", err)
		return nil, err
	}

	clientset, err := cm.getDynamicClient(env.Kubeconfig)
	if err != nil {
		cm.log.Error("Failed to get clientset, err: %v", err)
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "jdosclusters",
	}

	clusterList, err := clientset.Resource(gvr).Namespace("jd-tpaas").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		cm.log.Error("Failed to list JDOSClusters, err: %v", err)
		return nil, err
	}

	var clusters []ClusterInfo
	for _, cluster := range clusterList.Items {
		name := cluster.GetLabels()["cos.jdcloud.com/display-name"]
		os, _, _ := unstructured.NestedString(cluster.Object, "spec", "os")
		arch, _, _ := unstructured.NestedString(cluster.Object, "spec", "arch")
		region, _, _ := unstructured.NestedString(cluster.Object, "spec", "region")
		kubeconfig := cluster.GetLabels()["cos.jdcloud.com/kubeconfig-secret"]
		cri, _, _ := unstructured.NestedString(cluster.Object, "spec", "containerRuntime")
		version, _, _ := unstructured.NestedString(cluster.Object, "spec", "kubernetesVersion")
		apiServer, _, _ := unstructured.NestedMap(cluster.Object, "spec", "controlPlaneEndpoint")
		var apiServerURL string
		if apiServer != nil {
			ip, ok := apiServer["url"].(string)
			if ok {
				port, ok := apiServer["port"].(int64)
				if ok {
					apiServerURL = fmt.Sprintf("%s:%d", ip, port)
				}
			}
		}
		status, _, _ := unstructured.NestedBool(cluster.Object, "status", "ready")
		clusters = append(clusters, ClusterInfo{
			ID:         cluster.GetName(),
			Name:       name,
			OS:         os,
			ARCH:       arch,
			Region:     region,
			Cri:        cri,
			ApiServer:  apiServerURL,
			Version:    version,
			Kubeconfig: kubeconfig,
			Status:     strconv.FormatBool(status),
		})
	}
	return clusters, nil
}

func (cm *ClusterManager) ListClusterSecrets() ([]string, error) {
	cm.log.Info("Listing clusters for environment: %s", cm.EnvID)
	env, err := cm.getEnvironment()
	if err != nil {
		cm.log.Error("Failed to get environment: %v", err)
		return nil, err
	}

	clientset, err := cm.getClientset(env.Kubeconfig)
	if err != nil {
		cm.log.Error("Failed to get clientset: %v", err)
		return nil, err
	}

	secrets, err := clientset.CoreV1().Secrets("jd-tpaas").List(context.Background(), metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name",
	})
	if err != nil {
		cm.log.Error("Failed to list secrets: %v", err)
		return nil, fmt.Errorf("failed to list secrets: %v", err)
	}

	clusters := []string{"gaia"} // Add management cluster
	for _, secret := range secrets.Items {
		if secret.Type == "cluster.x-k8s.io/secret" && strings.HasSuffix(secret.Name, "-kubeconfig") {
			clusterName := secret.Labels["cluster.x-k8s.io/cluster-name"]
			clusters = append(clusters, clusterName)
		}
	}

	cm.log.Info("Found %d clusters", len(clusters))
	return clusters, nil
}

func (cm *ClusterManager) GetKubeconfig(clusterName string) (string, error) {
	cm.log.Info("Getting kubeconfig for cluster: %s", clusterName)
	env, err := cm.getEnvironment()
	if err != nil {
		cm.log.Error("Failed to get environment: %v", err)
		return "", err
	}

	if clusterName == "gaia" {
		cm.log.Info("Downloading kubeconfig for management cluster (gaia)")
		// Download the kubeconfig file from the management cluster
		sshClient := ssh.NewSSHClient(env.IP, env.User, env.Password)
		remotePath := "/root/.kube/config"
		localPath := filepath.Join(os.Getenv("HOME"), ".devctl", "kubeconfigs", cm.EnvID, "config")

		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			cm.log.Error("Failed to create directory: %v", err)
			return "", fmt.Errorf("failed to create directory: %v", err)
		}

		if err := sshClient.DownloadFile(remotePath, localPath); err != nil {
			cm.log.Error("Failed to download kubeconfig: %v", err)
			return "", fmt.Errorf("failed to download kubeconfig: %v", err)
		}

		cm.log.Info("Kubeconfig downloaded successfully")
		return localPath, nil
	}

	clientset, err := cm.getClientset(env.Kubeconfig)
	if err != nil {
		cm.log.Error("Failed to get clientset: %v", err)
		return "", err
	}

	secret, err := clientset.CoreV1().Secrets("jd-tpaas").Get(context.Background(), clusterName+"-kubeconfig", metav1.GetOptions{})
	if err != nil {
		cm.log.Error("Failed to get secret: %v", err)
		return "", fmt.Errorf("failed to get secret: %v", err)
	}

	kubeconfig, ok := secret.Data["value"]
	if !ok {
		cm.log.Error("Kubeconfig not found in secret")
		return "", fmt.Errorf("kubeconfig not found in secret")
	}

	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".devctl", "kubeconfigs", cm.EnvID, clusterName)
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0755); err != nil {
		cm.log.Error("Failed to create directory: %v", err)
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	if err := ioutil.WriteFile(kubeconfigPath, kubeconfig, 0600); err != nil {
		cm.log.Error("Failed to write kubeconfig: %v", err)
		return "", fmt.Errorf("failed to write kubeconfig: %v", err)
	}

	cm.log.Info("Kubeconfig saved successfully")
	return kubeconfigPath, nil
}

func (cm *ClusterManager) getEnvironment() (*config.Environment, error) {
	for _, env := range cm.Config.Envs {
		if env.ID == cm.EnvID {
			return &env, nil
		}
	}
	return nil, fmt.Errorf("environment not found: %s", cm.EnvID)
}

func (cm *ClusterManager) getClientset(kubeconfigPath string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	return clientset, nil
}

func (cm *ClusterManager) getDynamicClient(kubeconfigPath string) (*dynamic.DynamicClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %v", err)
	}

	c, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	return c, nil
}

func (cm *ClusterManager) DeleteCluster(clusterName string) error {
	cm.log.Info("Deleting cluster: %s", clusterName)
	if clusterName == "gaia" {
		cm.log.Error("Cannot delete management cluster (gaia)")
		return fmt.Errorf("cannot delete management cluster (gaia)")
	}

	env, err := cm.getEnvironment()
	if err != nil {
		cm.log.Error("Failed to get environment: %v", err)
		return err
	}

	clientset, err := cm.getClientset(env.Kubeconfig)
	if err != nil {
		cm.log.Error("Failed to get clientset: %v", err)
		return err
	}

	// Delete the Secret
	err = clientset.CoreV1().Secrets("jd-tpaas").Delete(context.Background(), clusterName+"-kubeconfig", metav1.DeleteOptions{})
	if err != nil {
		cm.log.Error("Failed to delete secret: %v", err)
		return fmt.Errorf("failed to delete secret: %v", err)
	}

	// Delete the local kubeconfig file
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".devctl", "kubeconfigs", cm.EnvID, clusterName)
	err = os.Remove(kubeconfigPath)
	if err != nil && !os.IsNotExist(err) {
		cm.log.Error("Failed to delete local kubeconfig: %v", err)
		return fmt.Errorf("failed to delete local kubeconfig: %v", err)
	}

	cm.log.Info("Cluster deleted successfully")
	return nil
}

func (cm *ClusterManager) AddCluster(clusterName, kubeconfig string) error {
	cm.log.Info("Adding new cluster: %s", clusterName)
	if clusterName == "gaia" {
		cm.log.Error("Cannot add management cluster (gaia)")
		return fmt.Errorf("cannot add management cluster (gaia)")
	}

	env, err := cm.getEnvironment()
	if err != nil {
		cm.log.Error("Failed to get environment: %v", err)
		return err
	}

	clientset, err := cm.getClientset(env.Kubeconfig)
	if err != nil {
		cm.log.Error("Failed to get clientset: %v", err)
		return err
	}

	// Check if the cluster already exists
	_, err = clientset.CoreV1().Secrets("jd-tpaas").Get(context.Background(), clusterName+"-kubeconfig", metav1.GetOptions{})
	if err == nil {
		cm.log.Error("Cluster %s already exists", clusterName)
		return fmt.Errorf("cluster %s already exists", clusterName)
	}

	// Create a new Secret to store the kubeconfig
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName + "-kubeconfig",
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": clusterName,
			},
		},
		Type: "cluster.x-k8s.io/secret",
		Data: map[string][]byte{
			"value": []byte(kubeconfig),
		},
	}

	_, err = clientset.CoreV1().Secrets("jd-tpaas").Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		cm.log.Error("Failed to create secret: %v", err)
		return fmt.Errorf("failed to create secret: %v", err)
	}

	// Save the kubeconfig to the local file system
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".devctl", "kubeconfigs", cm.EnvID, clusterName)
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0755); err != nil {
		cm.log.Error("Failed to create directory: %v", err)
		return fmt.Errorf("failed to create directory: %v", err)
	}

	if err := ioutil.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
		cm.log.Error("Failed to write kubeconfig: %v", err)
		return fmt.Errorf("failed to write kubeconfig: %v", err)
	}

	cm.log.Info("Cluster added successfully")
	return nil
}
