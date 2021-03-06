package traefiktemplater

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type processor struct {
	objType  runtime.Object
	queue    workqueue.RateLimitingInterface
	informer cache.SharedIndexInformer
	retries  int
	updater
}

type updater interface {
	addItem(obj interface{}) error
	delItem(obj interface{}) error
}

func makeProcessor(lw cache.ListerWatcher, obj runtime.Object, refresh time.Duration, updater updater) *processor {
	name := strings.Split(fmt.Sprintf("%T", obj), ".")
	queue := workqueue.NewNamed(strings.ToLower(name[1]))

	inf := cache.NewSharedIndexInformer(
		lw,
		obj,
		refresh,
		cache.Indexers{},
	)

	inf.AddEventHandler(makeQueueEventHandlers(queue))

	return &processor{
		objType:  obj,
		queue:    queue,
		informer: inf,
		updater:  updater,
	}
}

func makeQueueEventHandlers(queue workqueue.RateLimitingInterface) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
	}
}

func (p *processor) run(stopChan <-chan struct{}) {
	defer p.queue.ShutDown()
	go p.informer.Run(stopChan)

	<-stopChan
}

func (p *processor) hasSynced() bool {
	return p.informer.HasSynced()
}

func (p *processor) runWorker() {
	for p.processNextItem() {
	}
}

func (p *processor) processNextItem() bool {
	key, quit := p.queue.Get()
	if quit {
		return false
	}
	defer p.queue.Done(key)

	err := p.processItem(key.(string))

	if err == nil {
		p.queue.Forget(key)
	} else if p.queue.NumRequeues(key) < p.retries {
		p.queue.AddRateLimited(key)
	} else {
		p.queue.Forget(key)
		utilruntime.HandleError(err)
	}

	return true
}

func (p *processor) processItem(key string) error {
	obj, exists, err := p.informer.GetIndexer().GetByKey(key)
	if err != nil {
		return errors.Wrap(err, "failed calling the API")
	}
	if !exists {
		return p.delItem(obj)
	}
	return p.addItem(obj)
}
