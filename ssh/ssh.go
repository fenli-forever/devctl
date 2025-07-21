package ssh

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	Host     string
	User     string
	Password string
}

func NewSSHClient(host, user, password string) *SSHClient {
	return &SSHClient{
		Host:     host,
		User:     user,
		Password: password,
	}
}

func (c *SSHClient) Connect() (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: c.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(c.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", c.Host), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %v", err)
	}

	return client, nil
}

func (c *SSHClient) DownloadFile(remotePath, localPath string) error {
	client, err := c.Connect()
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	// Create a new file for writing
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer localFile.Close()

	// Set up the remote command
	remoteCmd := fmt.Sprintf("cat %s", remotePath)
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	// Start the remote command
	if err := session.Start(remoteCmd); err != nil {
		return fmt.Errorf("failed to start remote command: %v", err)
	}

	// Copy the output from the remote command to the local file
	_, err = io.Copy(localFile, stdout)
	if err != nil {
		return fmt.Errorf("failed to write to local file: %v", err)
	}

	// Wait for the remote command to finish
	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to wait for remote command: %v", err)
	}

	return nil
}

func (c *SSHClient) TestConnection() error {
	client, err := c.Connect()
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	_, err = session.Output("echo 'Connection successful'")
	if err != nil {
		return fmt.Errorf("failed to execute test command: %v", err)
	}

	return nil
}
