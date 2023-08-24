package monitoring

import (
	"context"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

type AppConfig struct {
	MetricsClientSet *versioned.Clientset
	MetricsMutex     *sync.Mutex
	Namespace        string
	CheckRAM         bool
	Threshold        int
	Period           int
	Clientset        *kubernetes.Clientset
	PreviousMetrics  map[string]int64
	Ctx              context.Context
	Deployment       string
}

func NewAppConfig(mmutex *sync.Mutex, ns string, checkedRam bool, t, p int, prevMetrics map[string]int64, depl string) *AppConfig {
	return &AppConfig{

		MetricsMutex:    mmutex,
		Namespace:       ns,
		CheckRAM:        checkedRam,
		Threshold:       t,
		Period:          p,
		PreviousMetrics: prevMetrics,
		Deployment:      depl,
	}

}
