package auth_test

import (
	"github.com/p-lewis/awsgolang/auth"
	"os"
	"testing"
)

const (
	ACCESS_KEY = "testAccessKey"
	SECRET_KEY = "testSecretKey"
)

func setEnv() {
	os.Setenv(auth.AWS_ACCESS_KEY_ID, ACCESS_KEY)
	os.Setenv(auth.AWS_SECRET_ACCESS_KEY, SECRET_KEY)
}

func delEnv() {
	os.Setenv(auth.AWS_ACCESS_KEY_ID, "")
	os.Setenv(auth.AWS_SECRET_ACCESS_KEY, "")
}

func TestGetCredentialsFromEnv(t *testing.T) {
	setEnv()
	c, err := auth.EnvCredentials()
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	if c.AccessKey != ACCESS_KEY {
		t.Errorf("AccessKey = %v, want %v", c.AccessKey, ACCESS_KEY)
	}
	if c.SecretKey != SECRET_KEY {
		t.Errorf("SecretKey = %v, want %v", c.SecretKey, SECRET_KEY)
	}
}

func TestErrForMissingAccessKey(t *testing.T) {
	delEnv()
	c, err := auth.EnvCredentials()
	if err == nil {
		t.Errorf("Expected an error, got nil.")
	}
	if c != nil {
		t.Errorf("Expected a nil Auth, got %v", c)
	}
	//t.Logf("Got expected error: %v", err)
}

func TestErrForMissingSecretKey(t *testing.T) {
	setEnv()
	os.Setenv(auth.AWS_SECRET_ACCESS_KEY, "")
	c, err := auth.EnvCredentials()
	if err == nil {
		t.Errorf("Expected an error, got nil.")
	}
	if c != nil {
		t.Errorf("Expected a nil Auth, got %v", c)
	}
	//t.Logf("Got expected error: %v", err)
}
