package controller

import (
	"context"
	"fmt"
	"log"
	"sync"

	k8s "github.com/umegbewe/kubectl-multilog/internal/k8sclient"
	"github.com/umegbewe/kubectl-multilog/internal/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

type Controller struct {
	K8sClient       *k8s.Client
	informerFactory informers.SharedInformerFactory
	ctx             context.Context
	cancelFunc      context.CancelFunc

	NamespaceObservers []chan []*model.Namespace
	PodObservers       []chan []*model.Pod

	Namespace map[string]*model.Namespace
	Pods      map[string]*model.Pod

	mu    sync.RWMutex
	ready chan struct{}
}

func NewController(k8sClient *k8s.Client) *Controller {
	ctx, cancel := context.WithCancel(context.Background())
	factory := informers.NewSharedInformerFactory(k8sClient.Clientset, 0)

	c := &Controller{
		K8sClient:          k8sClient,
		informerFactory:    factory,
		ctx:                ctx,
		cancelFunc:         cancel,
		NamespaceObservers: []chan []*model.Namespace{},
		PodObservers:       []chan []*model.Pod{},
		Namespace:          make(map[string]*model.Namespace),
		Pods:               make(map[string]*model.Pod),
		ready:              make(chan struct{}),
	}

	c.startInformers()
	return c
}

func (c *Controller) startInformers() {
	namespaceInformer := c.informerFactory.Core().V1().Namespaces().Informer()
	podInformer := c.informerFactory.Core().V1().Pods().Informer()

	namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onNamespaceAdd,
		UpdateFunc: c.onNamespaceUpdate,
		DeleteFunc: c.onNamespaceDelete,
	})

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onPodAdd,
		UpdateFunc: c.onPodUpdate,
		DeleteFunc: c.onPodDelete,
	})

	c.informerFactory.Start(c.ctx.Done())
	go func() {
		synced := c.informerFactory.WaitForCacheSync(c.ctx.Done())
		for informer, ok := range synced {
			if !ok {
				log.Printf("[ERROR] Informer %v not synced", informer)
			}
		}
		close(c.ready)
	}()
	
}

func (c *Controller) Stop() {
	c.cancelFunc()
}

func (c *Controller) Ready() <-chan struct{} {
	return c.ready
}

func (c *Controller) onNamespaceAdd(obj interface{}) {
	namespace := obj.(*corev1.Namespace)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Namespace[namespace.Name] = &model.Namespace{
		Name: namespace.Name,
	}
	c.notifyNamespaceObservers()
}

func (c *Controller) onNamespaceUpdate(oldObj, newObj interface{}) {
	c.onNamespaceAdd(newObj)
}

func (c *Controller) onNamespaceDelete(obj interface{}) {
	namespace := obj.(*corev1.Namespace)
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.Namespace, namespace.Name)
	c.notifyNamespaceObservers()
}

func (c *Controller) onPodAdd(obj interface{}) {
	pod := obj.(*corev1.Pod)
	c.mu.Lock()
	defer c.mu.Unlock()

	model := &model.Pod{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Containers: []string{},
	}

	for _, container := range pod.Spec.Containers {
		model.Containers = append(model.Containers, container.Name)
	}

	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	c.Pods[key] = model
	c.notifyPodObservers()
}

func (c *Controller) onPodUpdate(oldObj, newObj interface{}) {
	c.onPodAdd(newObj)
}

func (c *Controller) onPodDelete(obj interface{}) {
	pod := obj.(*corev1.Pod)
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	delete(c.Pods, key)
	c.notifyPodObservers()
}

func (c *Controller) RegisterNamespaceObserver(ch chan []*model.Namespace) {
    <-c.Ready() 
    c.mu.Lock()

    var list []*model.Namespace
    for _, ns := range c.Namespace {
        list = append(list, ns)
    }

    ch <- list

    c.NamespaceObservers = append(c.NamespaceObservers, ch)
    c.mu.Unlock()
}

func (c *Controller) RegisterPodObserver(ch chan []*model.Pod) {
    <-c.Ready() 
    c.mu.Lock()

    var list []*model.Pod
    for _, p := range c.Pods {
        list = append(list, p)
    }

    ch <- list

    c.PodObservers = append(c.PodObservers, ch)

    c.mu.Unlock()
}


func (c *Controller) notifyNamespaceObservers() {
    var namespaces []*model.Namespace
    for _, ns := range c.Namespace {
        namespaces = append(namespaces, ns)
    }

    for _, ch := range c.NamespaceObservers {
        ch <- namespaces
    }
}

func (c *Controller) notifyPodObservers() {
    var pods []*model.Pod
    for _, p := range c.Pods {
        pods = append(pods, p)
    }

    for _, ch := range c.PodObservers {
        ch <- pods
    }
}

