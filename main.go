package main

import (
	"context"
	"flag"
	"fmt"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/metrics/pkg/client/clientset/versioned"
	"sync"
	"time"
)

var kubeconfigPath, namespace string
var dryRun bool
var period, threshold int
var checkRAM bool
var previousMetrics map[string]int64
var metricsMutex sync.Mutex

func init() {
	flag.StringVar(&kubeconfigPath, "kubeconfig", "/path/to/.kube/config", "Path to kubeconfig file")
	flag.StringVar(&namespace, "namespace", "default", "Namespace to watch")
	flag.BoolVar(&dryRun, "dry-run", false, "If true, will only log the stale pods without deleting them")
	flag.IntVar(&period, "period", 60, "Time period in seconds to check for stale pods")
	flag.IntVar(&threshold, "threshold", 100, "Threshold for considering a pod stale")
	flag.BoolVar(&checkRAM, "check-ram", false, "Check RAM instead of CPU")
	previousMetrics = make(map[string]int64)
	flag.Parse()
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

func monitorStalePods(clientset *kubernetes.Clientset, metricsClientset *versioned.Clientset) {
	ticker := time.NewTicker(time.Duration(period) * time.Second)
	for range ticker.C {
		pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
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

func main() {
	clientset, metricsClientset, err := getK8sClient(kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}

	// Regularly clean up previousMetrics for deleted pods
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			metricsMutex.Lock()
			for podName := range previousMetrics {
				_, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
				if err != nil {
					delete(previousMetrics, podName)
				}
			}
			metricsMutex.Unlock()
		}
	}()

	monitorStalePods(clientset, metricsClientset)
}