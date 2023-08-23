package monitoring

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

func (app *AppConfig) IsStaledPod(podName string) bool {
	for retries := 0; retries < 3; retries++ {
		metrics, err := app.MetricsClientSet.MetricsV1beta1().PodMetricses(app.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err == nil {
			var currentUsage int64
			for _, container := range metrics.Containers {
				if app.CheckRAM {
					currentUsage += container.Usage.Memory().Value()
				} else {
					currentUsage += container.Usage.Cpu().MilliValue()
				}
			}

			app.MetricsMutex.Lock()
			defer app.MetricsMutex.Unlock()

			previousUsage, exists := app.PreviousMetrics[podName]
			if !exists {
				app.PreviousMetrics[podName] = currentUsage
				return false
			}

			delta := currentUsage - previousUsage
			app.PreviousMetrics[podName] = currentUsage

			return delta < int64(app.Threshold)
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Printf("Failed to get metrics for pod %s after retries\n", podName)
	return false
}

func (app *AppConfig) MonitorStalePods(dryRun bool) {
	ticker := time.NewTicker(time.Duration(app.Period) * time.Second)
	for range ticker.C {
		pods, err := app.Clientset.CoreV1().Pods(app.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Println("Failed to get pods:", err)
			continue
		}
		for _, pod := range pods.Items {
			if app.IsStaledPod(pod.Name) {
				fmt.Println("Stale pod detected:", pod.Name)
				if !dryRun {
					err := app.Clientset.CoreV1().Pods(app.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
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

func (app *AppConfig) GetK8sClient(kubeconfig string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	metricsClientset, err := versioned.NewForConfig(config)
	if err != nil {
		return err
	}

	app.Clientset = clientset
	app.MetricsClientSet = metricsClientset
	return nil
}
