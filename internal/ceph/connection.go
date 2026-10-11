package ceph

import (
	"fmt"
	"os"
	"errors"
	"path/filepath"
	"strings"
	"bytes"
)

const (
	configFileName  = "ceph.conf"
	keyringFileName = "keyring"
)

type ConnectionInput struct {
	ClusterID  string
	ClientName string

	ConfigData  []byte
	KeyringData []byte
}

type PreparedConnection struct {
	clusterID  string
	clientName string

	directory   string
	configPath  string
	keyringPath string
}

func clearSecretBytes(value []byte) {
		clear(value)
}

func PrepareConnection(input ConnectionInput) (*PreparedConnection, error) {
	if strings.TrimSpace(input.ClusterID) == "" {
		return nil, fmt.Errorf("Ceph Cluster ID is required")
	}

	if strings.TrimSpace(input.ClientName) == "" {
		return nil, fmt.Errorf("Ceph client name is required")
	}

	if len(input.ConfigData) == 0 {
		return nil, fmt.Errorf("Ceph configuration is required")
	}

	if len(input.KeyringData) == 0 {
		return nil, fmt.Errorf("Ceph keyring is required")
	}

	directory, err := os.MkdirTemp("", "pook-ceph-")
	if err != nil {
		return nil, fmt.Errorf("create private Ceph connection directory: %w", err)
	}

	connection := &PreparedConnection{
		clusterID:  input.ClusterID,
		clientName: input.ClientName,
		directory:  directory,
	}

	connection.configPath = filepath.Join(directory, configFileName)
	connection.keyringPath = filepath.Join(directory, keyringFileName)

	if err := writePrivateFile(connection.configPath, input.ConfigData); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("write ceph configuration: %w", err)
	}

	if err := writePrivateFile(connection.keyringPath, input.KeyringData); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("write ceph keyring: %w", err)
	}
	return connection, nil
}

func (c *PreparedConnection) ClusterID() string {
	return c.clusterID
}

func (c *PreparedConnection) ClientName() string {
	return c.clientName
}

func (c *PreparedConnection) ConfigPath() string {
	return c.configPath
}

func (c *PreparedConnection) KeyringPath() string {
	return c.keyringPath
}

func (c *PreparedConnection) Command(binary string, args ...string) Command {
	connectionArgs := []string{
		"--conf", c.configPath,
		"--keyring", c.keyringPath,
		"--name", c.clientName,
	}

	finalArgs := make([]string, 0, len(connectionArgs)+len(args))
	finalArgs = append(finalArgs, connectionArgs...)
	finalArgs = append(finalArgs, args...)

	return Command{
		Binary: binary,
		Args:   finalArgs,
	}
}

func (c *PreparedConnection) Close() error {
	if c == nil || c.directory == "" {
		return nil
	}

	err := os.RemoveAll(c.directory)
	c.directory = ""

	return err
}

func (c *PreparedConnection) WriteBootstrapToken(token []byte) (string, error) {
	if c == nil {
		return "", errors.New("Prepared connection is nil")
	}

	token = bytes.TrimSpace(token)
	if len(token) == 0 {
		return "", errors.New("Bootstrap token is empty")
	}

	if c.directory == "" {
		return "", errors.New("prepared connection is closed")
	}

	file, err := os.CreateTemp(c.directory, "bootstrap-token-*")
	if err != nil{
		return "", fmt.Errorf("Create bootstrap token file: %w", err)
	}

	tokenPath := file.Name()

	removeFile := func() {
		_ = file.Close()
		_ = os.Remove(tokenPath)
	}

	if err := file.Chmod(0o600); err != nil {
		removeFile()
		return "", fmt.Errorf("Set bootstrap token file permissions: %w", err)
	}

	if _, err := file.Write(token); err != nil {
		removeFile()
		return "", fmt.Errorf("Write bootstrap token file: %w", err)
	}

	if err := file.Sync(); err != nil {
		removeFile()
		return "", fmt.Errorf("Sync bootstrap token file: %w", err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tokenPath)
		return "", fmt.Errorf("Close bootstrap token file: %w", err)
	}
	return tokenPath, nil
}

func writePrivateFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}

	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return os.Chmod(path, 0600)
}
