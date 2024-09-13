package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/ish-xyz/registry-cache/pkg/cache"
	"github.com/ish-xyz/registry-cache/pkg/gc"
	"github.com/ish-xyz/registry-cache/pkg/metrics"
	"github.com/ish-xyz/registry-cache/pkg/proxy"

	"github.com/ish-xyz/registry-cache/pkg/worker"
	"github.com/inhies/go-bytesize"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	configFile string
	debug      bool
	trace      bool
	rootCmd    = &cobra.Command{
		Short: "registry-cache",
		Use:   "Run registry cache web server",
		Run:   start,
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "pass the config file path")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "run in debug mode")
	rootCmd.Flags().BoolVarP(&trace, "trace", "t", false, "run in trace mode")

	rootCmd.MarkFlagRequired("config")
}

func getHttpClientWithCA(capath string) (*http.Client, error) {
	caCert, err := ioutil.ReadFile(capath)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	return client, nil
}

func start(c *cobra.Command, args []string) {

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if trace {
		logrus.SetLevel(logrus.TraceLevel)
	}

	logrus.Infoln("loading and validating config...")
	cfg, err := LoadAndValidateConfig(configFile)
	if err != nil {
		logrus.Fatal("failed to load/validate config: \n", err)
	}

	logrus.Infoln("configuration:")
	fmt.Println("GOMAXPROCS =", runtime.GOMAXPROCS(0))
	yamlData, err := yaml.Marshal(cfg)
	if err == nil {
		fmt.Println(string(yamlData))
	} else {
		logrus.Errorln("can't print config")
	}

	logrus.Infoln("initializing cache...")
	indexObj := cache.NewMemoryIndex()
	err = os.MkdirAll(cfg.DataPath, 0777)
	if err != nil {
		logrus.Fatalln("failed to create folder for data", err)
	}

	cacheObj := cache.NewCache(indexObj, cfg.DataPath)
	err = cacheObj.Restore()
	if err != nil {
		logrus.Warningln("failed to restore index:", err)
	}

	logrus.Infoln("initializing garbageCollector...")
	maxSize, _ := bytesize.Parse(cfg.GC.Disk.MaxSize)
	gcObj := gc.NewGarbageCollector(
		cacheObj,
		indexObj,
		maxSize,
		cfg.GC.Layers.CheckSHA,
		cfg.GC.Interval,
		cfg.GC.Manifests.MaxAge,
		cfg.GC.Manifests.MaxUnused,
		cfg.GC.Layers.MaxAge,
		cfg.GC.Layers.MaxUnused,
	)

	logrus.Infoln("initializing  workers...")
	httpClient, err := getHttpClientWithCA(cfg.Server.TLS.CAPath)
	if err != nil {
		logrus.Fatalln("error loading CA:", err)
	}

	workerObj := worker.NewWorker(cacheObj, indexObj, httpClient, gcObj)

	logrus.Infoln("initializing  proxy...")
	urules, err := getUpstreamRules(cfg.Server.UpstreamRules)
	if err != nil {
		logrus.Fatalln(err)
	}
	proxyObj := proxy.NewProxy(
		workerObj,
		cfg.Server.Address,
		cfg.DataPath,
		cfg.Server.DefaultBackend.Host,
		cfg.Server.DefaultBackend.Scheme,
		cfg.Server.TLS.CertPath,
		cfg.Server.TLS.KeyPath,
		urules,
	)
	go metrics.Run(cfg.Metrics.Address, indexObj)
	go gcObj.Start()

	proxyDone := &sync.WaitGroup{}
	proxyDone.Add(1)

	srv := proxyObj.Start(cfg.Server.Workers, debug, proxyDone)

	// set up signal capturing
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	signal.Notify(stop, syscall.SIGTERM)

	// Waiting for SIGINT (kill -2)
	<-stop

	// now close the server gracefully
	logrus.Infoln("gracefully shutting down the proxy...")
	if err := srv.Shutdown(context.TODO()); err != nil {
		logrus.Fatalln(err)
	}

	proxyDone.Wait()

	logrus.Infoln("shutting down proxy: done")
}
