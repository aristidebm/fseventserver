package fseventserver

import (
	"os"
	"path/filepath"
	"strings"
)

func expandUser(path string) (string, error) {
	tilde := "~"
	if strings.HasPrefix(path, tilde) {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = strings.TrimPrefix(path, tilde)
		path = strings.TrimPrefix(path, string(filepath.Separator))
		userHome = strings.TrimSuffix(userHome, string(filepath.Separator))
		path = filepath.Join(userHome, path)
		return path, nil
	}
	return path, nil
}
