package identity

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type ClientIdentity struct {
	ClientID    string `json:"clientId"`
	DisplayName string `json:"displayName"`
}

func Load() (*ClientIdentity, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return generate()
		}
		return nil, err
	}

	var id ClientIdentity
	if err := json.Unmarshal(data, &id); err != nil {
		return generate()
	}
	if id.ClientID == "" {
		return generate()
	}
	return &id, nil
}

func Save(id *ClientIdentity) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func generate() (*ClientIdentity, error) {
	hostname, _ := os.Hostname()
	name := os.Getenv("USER")
	if name == "" {
		name = hostname
	}

	id := &ClientIdentity{
		ClientID:    "client_" + uuid.New().String()[:8],
		DisplayName: name,
	}
	if err := Save(id); err != nil {
		return id, nil
	}
	return id, nil
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zero", "client.json")
}
