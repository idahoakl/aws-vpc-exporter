package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/idahoakl/aws-vpc-exporter/pkg/config"
	"github.com/idahoakl/aws-vpc-exporter/pkg/subnet"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile      string
	envVarPrefix string
	listenAddr   string
	RootCmd      = cobra.Command{
		Use:     "aws_vpc_exporter",
		Args:    cobra.NoArgs,
		RunE:    run,
		Version: "N/A",
	}
)

var (
	// Revision represents the git sha of the source code for the binary
	Revision = "N/A"
	Version  = "edge"
	Branch   = "main"
)

func init() {
	RootCmd.SetVersionTemplate(fmt.Sprintf("version=%s  branch=%s  revision=%s\n", Version, Branch, Revision))
	version.Version = Version
	version.Revision = Revision
	version.Branch = Branch
	prometheus.MustRegister(version.NewCollector("aws_vpc_exporter"))

	RootCmd.Flags().StringVarP(&cfgFile, "config", "c", "config.yaml", "configuration file")
	RootCmd.Flags().StringVar(&envVarPrefix, "envPrefix", "SVC", "environment variable prefix to use for config values")
	RootCmd.Flags().StringSlice("subnetIds", []string{}, "subnet ids to retrieve")
	RootCmd.Flags().StringVarP(&listenAddr, "listenAddr", "l", ":9094", "address to listen on for HTTP requests")

	err := viper.BindPFlag("subnet.filter.ids", RootCmd.Flags().Lookup("subnetIds"))
	if err != nil {
		panic(err)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	var cfg config.Config
	var err error

	if !path.IsAbs(cfgFile) {
		viper.AddConfigPath(".")
	}
	viper.SetConfigFile(cfgFile)
	viper.SetEnvPrefix(envVarPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err = viper.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			log.Warn("configuration file not found, using defaults")
		} else {
			return err
		}
	}

	if err = viper.Unmarshal(&cfg); err != nil {
		return err
	}

	// config parsing done, don't show help usage on application errors
	cmd.SilenceUsage = true

	sess := session.Must(session.NewSession())

	svc := ec2.New(sess)

	if cfg.Subnet != nil {
		log.Info("Adding subnet collector")
		subnetCollector, err := subnet.New(svc, *cfg.Subnet)
		if err != nil {
			return err
		}

		prometheus.MustRegister(subnetCollector)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>AWS VPC Exporter</title></head>
             <body>
             <h1>AWS VPC Exporter</h1>
             <p><a href='/metrics'>Metrics</a></p>
             </body>
             </html>`))
	})
	server := http.Server{Addr: listenAddr, Handler: mux}

	exitChan := make(chan string)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		exitChan <- fmt.Sprint(<-c)
	}()

	log.Info("Listening on", listenAddr)
	go server.ListenAndServe()

	<-exitChan

	log.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	return nil
}
