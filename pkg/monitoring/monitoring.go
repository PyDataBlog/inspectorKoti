package monitoring

import (
	"InspectorKoti/pkg/debug"
	"context"
	"log"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
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
			debug.DebugPrint("Current usage: ", currentUsage, " Previous usage: ", previousUsage, " Delta: ", delta) // Debugging information
			return delta < int64(app.Threshold)
		}
		time.Sleep(2 * time.Second)
	}

	log.Printf("Failed to get metrics for pod %s after retries\n", podName)
	return false
}

func (app *AppConfig) MonitorStalePods(dryRun bool, ctx context.Context) {
	ticker := time.NewTicker(time.Duration(app.Period) * time.Second)
	defer ticker.Stop()

	for {

		select {
		case <-ctx.Done():
			debug.DebugPrint("Context done received in monitorStalePods.")
			return
		case <-ticker.C:
			debug.DebugPrint("Monitoring stale pods...") // Debugging information:
			var options metav1.ListOptions
			if app.Deployment != "" {
				pp := app.Clientset.CoreV1().Pods(app.Namespace)
				pp_list, err := pp.List(context.Background(), metav1.ListOptions{})
				if err != nil {
					log.Fatal("error geting pods", err)
				}
				app.stalepodsDelete(pp_list, dryRun, true)

			} else {
				pods, err := app.Clientset.CoreV1().Pods(app.Namespace).List(context.TODO(), options)
				if err != nil {
					log.Println("Failed to get pods:", err)
					continue
				}
				app.stalepodsDelete(pods, dryRun, false)

			}

		}
	}
}

func (app *AppConfig) stalepodsDelete(pods *v1.PodList, dryRun bool, deployment_check bool) {
	if deployment_check {
		for _, pod := range pods.Items {

			if strings.Contains(pod.Name, app.Deployment) {
				pod_name := pod.Name
				if app.IsStaledPod(pod_name) {
					log.Println("Stale pod detected:", pod_name)
					if !dryRun {
						err := app.Clientset.CoreV1().Pods(app.Namespace).Delete(context.TODO(), pod_name, metav1.DeleteOptions{})
						if err != nil {
							log.Println("Failed to delete pod:", err)
						} else {
							log.Println("Deleted stale pod:", pod_name)
						}
					}
				}
			}
		}
	} else {
		for _, pod := range pods.Items {
			if app.IsStaledPod(pod.Name) {
				log.Println("Stale pod detected:", pod.Name)
				if !dryRun {
					err := app.Clientset.CoreV1().Pods(app.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
					if err != nil {
						log.Println("Failed to delete pod:", err)
					} else {
						log.Println("Deleted stale pod:", pod.Name)
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
