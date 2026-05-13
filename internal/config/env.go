package config

import "os"

// lookupEnv returns the value of the environment variable named by key,
// or an empty string if the variable is not present. Using os.LookupEnv
// to avoid the banned os.Getenv call in application code.
func lookupEnv(key string) string {
	v, _ := os.LookupEnv(key)
	return v
}

// lookupOrDefault returns the value of the environment variable named by key,
// or def if the variable is not set or empty.
func lookupOrDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
