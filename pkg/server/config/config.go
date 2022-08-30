package config

type Config struct {
	BindAddress string `json:"bindAddress"`
	BindPort    int    `json:"bindPort"`
	TlsCert     string `json:"tlsCert"`
	TlsKey      string `json:"tlsKey"`
}

func NewServerConfig() *Config {
	return &Config{}
}
