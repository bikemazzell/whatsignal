package database

import "os"

func init() {
	_ = os.Setenv("WHATSIGNAL_ENV", "development")
}
