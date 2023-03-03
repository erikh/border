package config

import (
	"errors"
	"os"
	"testing"
)

func errorSave() ([]byte, error) {
	return nil, errors.New("intentional error")
}

func errorLoad(data []byte) (Config, error) {
	return Config{}, errors.New("intentional error")
}

func TestErrors(t *testing.T) {
	if err := ToDisk(os.DevNull, errorSave); err == nil {
		t.Fatal("ToDisk did not error")
	}

	if _, err := FromDisk(os.DevNull, errorLoad); err == nil {
		t.Fatal("FromDisk did not error")
	}
}
