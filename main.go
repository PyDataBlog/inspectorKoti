package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

var kubeconfigPath, namespace, targetDeployment string
var dryRun, debug, checkRAM bool
var period, threshold, timeout int
var previousMetrics map[string]int64
var metricsMutex sync.Mutex

func init() {
	flag.StringVar(&kubeconfigPath, "kubeconfig", "/path/to/.kube/config", "Path to kubeconfig file")
	flag.StringVar(&namespace, "namespace", "default", "Namespace to watch")
	flag.StringVar(&targetDeployment, "deployment", "", "Target deployment to watch")
	flag.BoolVar(&dryRun, "dry-run", false, "If true, will only log the stale pods without deleting them")
	flag.IntVar(&period, "period", 60, "Time period in seconds to check for stale pods")
	flag.IntVar(&threshold, "threshold", 100, "Threshold for considering a pod stale")
	flag.IntVar(&timeout, "timeout", 0, "Timeout in seconds for the program to run. Default is 0 (indefinite).")
	flag.BoolVar(&checkRAM, "check-ram", false, "Check RAM instead of CPU")
	flag.BoolVar(&debug, "debug", false, "Enable or disable debug mode")
	previousMetrics = make(map[string]int64)
}

func debugPrint(v ...interface{}) {
	if debug {
		fmt.Println(v...)
	}
}

func isStalePod(podName, namespace string, metricsClientset *versioned.Clientset) bool {
	for retries := 0; retries < 3; retries++ {
		metrics, err := metricsClientset.MetricsV1beta1().PodMetricses(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err == nil {
			var currentUsage int64
			for _, container := range metrics.Containers {
				if checkRAM {
					currentUsage += container.Usage.Memory().Value()
				} else {
					currentUsage += container.Usage.Cpu().MilliValue()
				}
			}

			metricsMutex.Lock()
			defer metricsMutex.Unlock()

			previousUsage, exists := previousMetrics[podName]
			if !exists {
				previousMetrics[podName] = currentUsage
				return false
			}

			delta := currentUsage - previousUsage
			previousMetrics[podName] = currentUsage

			return delta < int64(threshold)
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Printf("Failed to get metrics for pod %s after retries\n", podName)
	return false
}

func getK8sClient(kubeconfig string) (*kubernetes.Clientset, *versioned.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	metricsClientset, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return clientset, metricsClientset, nil
}

func monitorStalePods(ctx context.Context, clientset *kubernetes.Clientset, metricsClientset *versioned.Clientset) {
	ticker := time.NewTicker(time.Duration(period) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			debugPrint("Context done received in monitorStalePods.")
			return
		case <-ticker.C:
			debugPrint("Monitoring stale pods...") // Debugging information
			var options metav1.ListOptions
			if targetDeployment != "" {
				deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), targetDeployment, metav1.GetOptions{})
				if err != nil {
					fmt.Println("Failed to get deployment:", err)
					continue
				}
				labelSelector := labels.Set(deployment.Spec.Selector.MatchLabels).String()
				options = metav1.ListOptions{LabelSelector: labelSelector}
			}
			pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), options)
			if err != nil {
				fmt.Println("Failed to get pods:", err)
				continue
			}
			for _, pod := range pods.Items {
				if isStalePod(pod.Name, namespace, metricsClientset) {
					fmt.Println("Stale pod detected:", pod.Name)
					if !dryRun {
						err := clientset.CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
						if err != nil {
							fmt.Println("Failed to delete pod:", err)
						} else {
							fmt.Println("Deleted stale pod:", pod.Name)
						}
					}
				}
			}
		}
	}
}

func main() {
	flag.Parse()
	debugPrint("Raw args:", os.Args) // Debugging information
	flag.VisitAll(func(f *flag.Flag) {
		debugPrint(f.Name, ": ", f.Value)
	}) // Debugging information
	debugPrint("Parsed timeout value:", timeout) // Debugging information
	clientset, metricsClientset, err := getK8sClient(kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}

	debugPrint("Timeout value:", timeout) // Debugging information to print the timeout value

	ctx, cancel := context.WithCancel(context.Background())

	go monitorStalePods(ctx, clientset, metricsClientset)

	if timeout > 0 {
		go func() {
			debugPrint("Starting timeout countdown...") // Debugging information
			time.Sleep(time.Duration(timeout) * time.Second)
			debugPrint("Timeout reached. Attempting to terminate program.") // Debugging information
			cancel()
		}()
	}

	<-ctx.Done()
	debugPrint("Program terminated.") // Debugging information
	os.Exit(0)
}
