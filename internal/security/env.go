package security

import "os"

const DevelopmentEnv = "development"

func IsSecureMode() bool {
	return os.Getenv("WHATSIGNAL_ENV") != DevelopmentEnv
}
