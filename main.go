package main

import (
	"InspectorKoti/pkg/debug"
	"InspectorKoti/pkg/monitoring"
	"context"
	"flag"
	"log"
	"os"
	"sync"
	"time"
)

var (
	kubeconfigPath, namespace, targetDeployment string
	dryRun, checkRAM, debugMode                 bool
	period, threshold, timeout                  int
	previousMetrics                             map[string]int64
	metricsMutex                                sync.Mutex
)

func init() {
	flag.StringVar(&kubeconfigPath, "kubeconfig", "/path/to/.kube/config", "Path to kubeconfig file")
	flag.StringVar(&namespace, "namespace", "default", "Namespace to watch")
	flag.StringVar(&targetDeployment, "deployment", "", "Target deployment to watch")
	flag.BoolVar(&dryRun, "dry-run", false, "If true, will only log the stale pods without deleting them")
	flag.IntVar(&period, "period", 60, "Time period in seconds to check for stale pods")
	flag.IntVar(&threshold, "threshold", 100, "Threshold for considering a pod stale")
	flag.IntVar(&timeout, "timeout", 0, "Timeout in seconds for the program to run. Default is 0 (indefinite).")
	flag.BoolVar(&checkRAM, "check-ram", false, "Check RAM instead of CPU")
	flag.BoolVar(&debugMode, "debug", false, "Enable debug mode")
	previousMetrics = make(map[string]int64)
	flag.Parse()
	debug.SetDebugMode(debugMode)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	debug.DebugPrint("Raw args:", os.Args) // Debugging information
	flag.VisitAll(func(f *flag.Flag) {
		debug.DebugPrint(f.Name, ": ", f.Value)
	}) // Debugging information
	debug.DebugPrint("Parsed timeout value:", timeout) // Debugging information

	app := monitoring.NewAppConfig(&metricsMutex, namespace, checkRAM, threshold, period, previousMetrics, targetDeployment)
	app.GetK8sClient(kubeconfigPath)
	debug.DebugPrint("Timeout value:", timeout) // Debugging information to print the timeout value

	ctx, cancel := context.WithCancel(context.Background())
	go app.MonitorStalePods(dryRun, ctx)

	if timeout > 0 {
		go func() {
			debug.DebugPrint("Starting timeout countdown...") // Debugging information
			time.Sleep(time.Duration(timeout) * time.Second)
			log.Println("Timeout reached. Attempting to terminate program.") // Debugging information
			cancel()
		}()
	}

	<-ctx.Done()
	debug.DebugPrint("Program terminated.")

}
