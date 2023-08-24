package main

import (
	"InspectorKoti/pkg/monitoring"
	"context"
	"flag"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kubeconfigPath, namespace, targetDeployment string
	dryRun, checkRAM                            bool
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
	previousMetrics = make(map[string]int64)
	flag.Parse()
}

func main() {

	app := monitoring.NewAppConfig(&metricsMutex, namespace, checkRAM, threshold, period, previousMetrics, targetDeployment)
	app.GetK8sClient(kubeconfigPath)

	// Regularly clean up previousMetrics for deleted pods
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			app.MetricsMutex.Lock()
			for podName := range app.PreviousMetrics {
				_, err := app.Clientset.CoreV1().Pods(app.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
				if err != nil {
					delete(app.PreviousMetrics, podName)
				}
			}
			app.MetricsMutex.Unlock()
		}
	}()

	app.MonitorStalePods(dryRun)
}
