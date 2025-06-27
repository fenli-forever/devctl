package ssh

import (
	"fmt"
	"io/ioutil"

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

	content, err := session.Output(fmt.Sprintf("cat %s", remotePath))
	if err != nil {
		return fmt.Errorf("failed to read remote file: %v", err)
	}

	err = ioutil.WriteFile(localPath, content, 0600)
	if err != nil {
		return fmt.Errorf("failed to write local file: %v", err)
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
