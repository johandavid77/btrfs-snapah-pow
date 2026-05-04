package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Btrfs    BtrfsConfig    `mapstructure:"btrfs"`
	Agent    AgentConfig    `mapstructure:"agent"`
}

type ServerConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	GRPCPort   int    `mapstructure:"grpc_port"`
	TLSEnabled bool   `mapstructure:"tls_enabled"`
	TLSCert    string `mapstructure:"tls_cert"`
	TLSKey     string `mapstructure:"tls_key"`
	TLSCACert  string `mapstructure:"tls_ca_cert"`
}

type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type AuthConfig struct {
	JWTSecret   string `mapstructure:"jwt_secret"`
	TokenExpiry int    `mapstructure:"token_expiry_hours"`
	MTLSEnabled bool   `mapstructure:"mtls_enabled"`
}

type BtrfsConfig struct {
	DefaultSnapshotDir string `mapstructure:"default_snapshot_dir"`
	MaxSnapshotSizeGB  int    `mapstructure:"max_snapshot_size_gb"`
}

type AgentConfig struct {
	ServerAddress string `mapstructure:"server_address"`
	NodeID        string `mapstructure:"node_id"`
	Token         string `mapstructure:"token"`
	HeartbeatSec  int    `mapstructure:"heartbeat_seconds"`
	TLSCert       string `mapstructure:"tls_cert"`
	TLSKey        string `mapstructure:"tls_key"`
	TLSCACert     string `mapstructure:"tls_ca_cert"`
}

func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetEnvPrefix("SNAPAH")
	viper.AutomaticEnv()

	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.grpc_port", 9090)
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("btrfs.default_snapshot_dir", ".snapshots")
	viper.SetDefault("agent.heartbeat_seconds", 30)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error leyendo config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error parseando config: %w", err)
	}

	return &cfg, nil
}
