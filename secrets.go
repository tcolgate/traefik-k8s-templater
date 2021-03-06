package traefiktemplater

import (
	"net/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	listcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type secretsSet map[serviceKey][]*url.URL

type secUpdater struct {
	c *Controller
}

func (*secUpdater) addItem(obj interface{}) error {
	return nil
}

func (*secUpdater) delItem(obj interface{}) error {
	return nil
}

func (c *Controller) setupSecretProcess() error {
	upd := &secUpdater{c}

	c.secProc = makeProcessor(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return c.client.CoreV1().Secrets(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return c.client.CoreV1().Secrets(metav1.NamespaceAll).Watch(options)
			},
		},
		&corev1.Secret{},
		c.refresh,
		upd,
	)

	c.secList = listcorev1.NewSecretLister(c.secProc.informer.GetIndexer())

	return nil
}

func (c *Controller) processSecItem(string) error {
	return nil
}
