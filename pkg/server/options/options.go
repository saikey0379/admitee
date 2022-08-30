package options

import (
	"admitee/pkg/server/config"
	"fmt"
	"github.com/spf13/pflag"
)

// Options used for admitee server
type Options struct {
	BindAddress   string
	BindPort      int
	TlsCert       string
	TlsKey        string
	RedisAddress  string
	RedisPort     int
	RedisDB       int
	RedisPassword string
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) ApplyTo(cfg *config.Config) error {
	cfg.BindAddress = o.BindAddress
	cfg.BindPort = o.BindPort
	cfg.TlsCert = o.TlsCert
	cfg.TlsKey = o.TlsKey

	return nil
}

func (o *Options) Validate() []error {
	var errors []error

	if o.BindPort < 0 || o.BindPort > 65535 {
		errors = append(
			errors,
			fmt.Errorf(
				"--server-bind-port %v must be between 0 and 65535, inclusive. 0 for turning off insecure (HTTP) port",
				o.BindPort,
			),
		)
	}

	return errors
}

// AddFlags adds flags related to features for a specific server option to the
// specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if fs == nil {
		return
	}

	fs.StringVar(&o.BindAddress, "server-bind-address", "0.0.0.0", ""+
		"The IP address on which to serve the --server-bind-port "+
		"(set to 0.0.0.0 for all IPv4 interfaces and :: for all IPv6 interfaces).")
	fs.IntVar(&o.BindPort, "server-bind-port", 443,
		"The port on which to serve unsecured, unauthenticated access")
	fs.StringVar(&o.TlsCert, "tls-cert", "/etc/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	fs.StringVar(&o.TlsKey, "tls-key", "/etc/certs/key.pem", "File containing the x509 private key to --tls-cert.")

	fs.StringVar(&o.RedisAddress, "redis-address", "127.0.0.1", "Redis for replicas share pod messages.")
	fs.IntVar(&o.RedisPort, "redis-port", 6379, "Redis port.")
	fs.IntVar(&o.RedisDB, "redis-db", 0, "Redis db number.")
	fs.StringVar(&o.RedisPassword, "redis-password", "test", "Redis password.")

}
