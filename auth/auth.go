// Manage authentication credentials.
package auth

import (
	"fmt"
	"os"
)

type Credentials struct {
	AccessKey, SecretKey string
}

const (
	AWS_ACCESS_KEY_ID     = "AWS_ACCESS_KEY_ID"
	AWS_SECRET_ACCESS_KEY = "AWS_SECRET_ACCESS_KEY"
)

// Retreives a Credentials struct from environment variables AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
func EnvCredentials() (cred *Credentials, err error) {
	accessKey := os.Getenv(AWS_ACCESS_KEY_ID)
	secretKey := os.Getenv(AWS_SECRET_ACCESS_KEY)
	if accessKey == "" {
		return nil, fmt.Errorf("auth.EnvCredentials: Could not find env variable %v", AWS_ACCESS_KEY_ID)
	} else if secretKey == "" {
		return nil, fmt.Errorf("auth.EnvCredentials: Could not find env variable %v", AWS_SECRET_ACCESS_KEY)
	}
	cred = new(Credentials)
	cred.AccessKey = accessKey
	cred.SecretKey = secretKey
	return
}
