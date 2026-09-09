// Package libraryauth contains the password format shared by the server and its setup command.
package libraryauth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const PasswordFile = ".admin-password"

func ReadHash(directory string) ([]byte, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := root.ReadFile(PasswordFile)
	if err != nil {
		return nil, err
	}
	hash := []byte(strings.TrimSpace(string(data)))
	if _, err := bcrypt.Cost(hash); err != nil {
		return nil, err
	}
	return hash, nil
}

func SetPassword(directory string, password []byte) error {
	if len(password) < 12 || len(password) > 72 {
		return errors.New("use a password between 12 and 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword(password, 12)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".password-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(append(hash, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(directory, PasswordFile))
}
