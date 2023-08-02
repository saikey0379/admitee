package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"admitee/pkg/server"
	"admitee/pkg/server/config"
	"admitee/pkg/server/options"
)

// main.
func main() {
	ctx := signals.SetupSignalHandler()

	if err := NewCommand(ctx).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// NewCommand creates a *cobra.Command object with default parameters
func NewCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "admiteed",
		Long: `The server us running for admission`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Validate(); err != nil {
				glog.Exitf("Opts validate failed: %v", err)
			}
			if err := Run(ctx, opts); err != nil {
				glog.Exit(err)
			}
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(cmd.Flags())

	return cmd
}

func Run(ctx context.Context, opts *options.Options) error {
	var eg errgroup.Group

	clientRedis, err := opts.NewClientRedis()
	if err != nil {
		glog.Errorf("FAILURE: NewClientRedis[%v]", err)

		panic(err)
	} else {
		glog.Infof("Initial ClientRedis.")
	}

	clientSmooth, err := NewClientSmooth()
	if err != nil {
		glog.Errorf("FAILURE: NewClientSmooth[%v]", err)

		panic(err)
	} else {
		glog.Infof("Initial ClientSmooth.")
	}

	clientKubeSet, err := NewClientKubeSet()
	if err != nil {
		glog.Errorf("FAILURE: NewClientKubeSet[%v]", err)

		panic(err)
	} else {
		glog.Infof("Initial ClientKubeSet.")
	}

	eg.Go(func() error {
		// Start admitee server
		serverConfig := config.NewServerConfig()
		if err := opts.ApplyTo(serverConfig); err != nil {
			glog.Exit(err)
		}
		server, err := server.NewServer(serverConfig, clientSmooth, clientKubeSet, clientRedis)
		if err != nil {
			glog.Exit(err)
		}

		server.Run(ctx)
		return nil
	})

	// wait for all components exit
	if err := eg.Wait(); err != nil {
		glog.Fatal(err)
	}
	return err
}

func NewClientSmooth() (dynamic.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	smooth, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return smooth, nil
}

func NewClientKubeSet() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}
