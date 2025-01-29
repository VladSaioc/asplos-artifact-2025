package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type PopProcessFunc func()

type ProcessFunc func()

func Util(f func(), stopCh <-chan struct{}) {
	JitterUntil(f, stopCh)
}

func JitterUntil(f func(), stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		default:
		}
		func() {
			f()
		}()
	}
}

type Queue interface {
	HasSynced()
	Pop(PopProcessFunc)
}

type Config struct {
	Queue
	Process ProcessFunc
}

type Controller struct {
	config Config
}

func (c *Controller) Run(stopCh <-chan struct{}) {
	Util(c.processLoop, stopCh)
}

func (c *Controller) HasSynced() {
	c.config.Queue.HasSynced()
}

func (c *Controller) processLoop() {
	c.config.Queue.Pop(PopProcessFunc(c.config.Process))
}

type ControllerInterface interface {
	Run(<-chan struct{})
	HasSynced()
}

type ResourceEventHandler interface {
	OnAdd()
}

type ResourceEventHandlerFuncs struct {
	AddFunc func()
}

func (r ResourceEventHandlerFuncs) OnAdd() {
	if r.AddFunc != nil {
		r.AddFunc()
	}
}

type informer struct {
	controller ControllerInterface

	stopChan chan struct{}
}

type federatedInformerImpl struct {
	sync.Mutex
	clusterInformer informer
}

func (f *federatedInformerImpl) ClustersSynced() {
	f.Lock() // L1
	defer f.Unlock()
	f.clusterInformer.controller.HasSynced()
}

func (f *federatedInformerImpl) addCluster() {
	f.Lock() // L1
	defer f.Unlock()
}

func (f *federatedInformerImpl) Start() {
	f.Lock() // L1
	defer f.Unlock()

	f.clusterInformer.stopChan = make(chan struct{})
	// deadlocks: x > 0
	go f.clusterInformer.controller.Run(f.clusterInformer.stopChan) // G2
	runtime.Gosched()
}

func (f *federatedInformerImpl) Stop() {
	f.Lock() // L1
	defer f.Unlock()
	close(f.clusterInformer.stopChan)
}

type DelayingDeliverer struct{}

func (d *DelayingDeliverer) StartWithHandler(handler func()) {
	go func() { // G4
		// deadlocks: x > 0
		handler()
	}()
}

type FederationView interface {
	ClustersSynced()
}

type FederatedInformer interface {
	FederationView
	Start()
	Stop()
}

type NamespaceController struct {
	namespaceDeliverer         *DelayingDeliverer
	namespaceFederatedInformer FederatedInformer
}

func (nc *NamespaceController) isSynced() {
	nc.namespaceFederatedInformer.ClustersSynced()
}

func (nc *NamespaceController) reconcileNamespace() {
	nc.isSynced()
}

func (nc *NamespaceController) Run(stopChan <-chan struct{}) {
	nc.namespaceFederatedInformer.Start()
	go func() { // G3
		// deadlocks: x > 0
		<-stopChan
		nc.namespaceFederatedInformer.Stop()
	}()
	nc.namespaceDeliverer.StartWithHandler(func() {
		nc.reconcileNamespace()
	})
}

type DeltaFIFO struct {
	lock sync.RWMutex
}

func (f *DeltaFIFO) HasSynced() {
	f.lock.Lock() // L2
	defer f.lock.Unlock()
}

func (f *DeltaFIFO) Pop(process PopProcessFunc) {
	f.lock.Lock() // L2
	defer f.lock.Unlock()
	process()
}

func NewFederatedInformer() FederatedInformer {
	federatedInformer := &federatedInformerImpl{}
	federatedInformer.clusterInformer.controller = NewInformer(
		ResourceEventHandlerFuncs{
			AddFunc: func() {
				federatedInformer.addCluster()
			},
		})
	return federatedInformer
}

func NewInformer(h ResourceEventHandler) *Controller {
	fifo := &DeltaFIFO{}
	cfg := &Config{
		Queue: fifo,
		Process: func() {
			h.OnAdd()
		},
	}
	return &Controller{config: *cfg}
}

func NewNamespaceController() *NamespaceController {
	nc := &NamespaceController{}
	nc.namespaceDeliverer = &DelayingDeliverer{}
	nc.namespaceFederatedInformer = NewFederatedInformer()
	return nc
}

func monitor() {
	var mem = runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	fmt.Println("Final goroutine count:", runtime.NumGoroutine())
}

func main() {
	defer func() {
		time.Sleep(time.Minute / 2)
		runtime.GC()

		monitor()
	}()

	go func() {
		for {
			time.Sleep(time.Second / 2)
			for i := 0; i < 100; i++ {
				go func() { // G1
					namespaceController := NewNamespaceController()
					stop := make(chan struct{})
					namespaceController.Run(stop)
					close(stop)
				}()
			}
		}
	}()
}

/// Example of deadlocking trace.
///
/// G1												G2										G3											G4
/// ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
/// namespaceController.Run()
/// nc.namespaceFederatedInformer.Start()
///	f.Lock() [L1]
/// go f.clusterInformer.controller.Run()[G2]
/// <<<Gosched>>>
///	.												Util(c.processLoop, stopCh)
///	.												c.config.Queue.Pop()
///	.												f.lock.Lock() [L2]
///	.												process() <c.config.Process()>
///	.												h.OnAdd()
///	.												r.AddFunc()
///	.												federatedInformer.addCluster()
///	.												f.Lock() [L1]
/// f.Unlock() [L1]									.
/// go func()[G3]									.
/// nc.namespaceDeliverer.StartWithHandler()		.										.
/// go func()[G4]									.										.
/// close(stop)										.										.											.
///	<<<done>>>										.										.											.
/// 												.										<-stopChan									.
///													.										nc.namespaceFederatedInformer.Stop()		.
///													.										f.Lock() [L1]								.
/// 												.										.											handler()
///													.										.											nc.reconcileNamespace()
///													.										.											nc.isSynced()
///													.										.											nc.namespaceFederatedInformer.ClustersSynced()
///													.										.											f.Lock() [L1]
///													.										.											f.clusterInformer.controller.HasSynced()
///													.										.											c.config.Queue.HasSynced()
///													.										.											f.lock.Lock() [L2]
///----------------------------------------------------------------------------G2,G3,G4 leak----------------------------------------------------------------------------------------------
///
