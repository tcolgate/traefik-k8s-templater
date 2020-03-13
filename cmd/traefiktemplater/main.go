package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	traefiktemplater "github.com/QubitProducts/traefik-k8s-templater"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

func init() {
}

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")

	httpAddr = flag.String("addr.http", ":80", "address to serve http")
)

func main() {
	flag.Parse()

	stop := setupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	adminMux := http.NewServeMux()

	registry := prometheus.NewRegistry()

	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	registry.MustRegister(prometheus.NewGoCollector())
	metricsprovider := traefiktemplater.NewPrometheusMetrics(registry)
	traefiktemplater.SetProvider(metricsprovider)

	adminMux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err.Error())
	}

	ctrl, err := traefiktemplater.New(clientset)
	if err != nil {
		log.Fatalf("error creating controller, err = %v", err)
		return
	}

	adminMux.Handle("/healthz", http.HandlerFunc(ctrl.ServeHealthzHTTP))
	adminMux.HandleFunc("/debug/pprof/", pprof.Index)
	adminMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	adminMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	adminMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	adminMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	go ctrl.Run(stop)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	server := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Addr:         *httpAddr,
		Handler:      ctrl,
	}
	g.Go(func() error {
		err := http.ListenAndServe(*httpAddr, adminMux)
		if err != nil {
			log.Printf("http listener error, %v", err)
		}
		return err
	})

	<-ctx.Done()
	server.Shutdown(context.Background())
	if err := ctx.Err(); err != nil {
		os.Exit(1)
	}
}
