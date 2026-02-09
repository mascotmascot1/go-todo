package config

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	envHost      = "TODO_HOST"
	envPort      = "TODO_PORT"
	envDBFile    = "TODO_DBFILE"
	envPassword  = "TODO_PASSWORD"
	envSecretKey = "TODO_SECRETKEY"
)

type server struct {
	Host   string
	Port   int
	WebDir string
	DBFile string
}

type Auth struct {
	TokenTTL     time.Duration
	Password     string
	PasswordHash string
	SecretKey    []byte
}

type Limits struct {
	TasksLimit    int
	MaxUploadSize int64
}

type Config struct {
	Server server
	Limits Limits
	Auth   Auth
}

// New returns a new Config instance with default values set.
// It also checks environment variables for setting up the server port, path to db, and secret key.
// If the password is set via the environment variable TODO_PASSWORD, but the secret key TODO_SECRETKEY is missing,
// it will return an error.
//
// TODO_PORT: sets the server port.
// TODO_DBFILE: sets the path to the database file.
// TODO_PASSWORD: sets the password for the authentication.
// TODO_SECRETKEY: sets the secret key for the authentication.
//
// The default values are:
// - Server: host = "127.0.0.1", port = 7540, web directory = "web", database file = "scheduler.db"
// - Limits: tasks limit = 50, max upload size = 8 MiB
// - Auth: token ttl = 8 hours, password hash calculated from TODO_PASSWORD, secret key = TODO_SECRETKEY
func New() (*Config, error) {
	password := os.Getenv(envPassword)
	secretKey := os.Getenv(envSecretKey)

	if password != "" && secretKey == "" {
		return nil, fmt.Errorf("password is set via %s, but secret key %s is missing", envPassword, envSecretKey)
	}

	var hashPasswordStr string
	if password != "" {
		hashPassword := sha512.Sum512([]byte(password))
		hashPasswordStr = hex.EncodeToString(hashPassword[:])
	}

	cfg := &Config{
		Server: server{
			Host:   "127.0.0.1",
			Port:   7540,
			WebDir: "web",
			DBFile: "scheduler.db",
		},
		Limits: Limits{
			TasksLimit:    50,
			MaxUploadSize: 8 << 20,
		},
		Auth: Auth{
			TokenTTL:     time.Hour * 8,
			Password:     password,
			PasswordHash: hashPasswordStr,
			SecretKey:    []byte(secretKey),
		},
	}
	// Check environment variable for setting up the path to db.
	if db := os.Getenv(envDBFile); db != "" {
		cfg.Server.DBFile = db
	}

	// Check environment variable for setting up host.
	if h := os.Getenv(envHost); h != "" {
		cfg.Server.Host = h
	}

	// Check environment variable for setting up port.
	if p := os.Getenv(envPort); p != "" {
		eport, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid port value in %s: %w", p, err)
		}
		cfg.Server.Port = eport
	}
	return cfg, nil
}
