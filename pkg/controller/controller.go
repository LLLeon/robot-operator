package controller

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslister "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	clientset "robot-operator/pkg/generated/clientset/versioned"
	robotinformers "robot-operator/pkg/generated/informers/externalversions/robot/v1"
	robotlisters "robot-operator/pkg/generated/listers/robot/v1"
)

type Controller struct {
	kubeClientset  kubernetes.Interface
	robotClientset clientset.Interface

	deploymentsLister appslister.DeploymentLister
	robotsLister      robotlisters.RobotLister

	deploymentsSynced cache.InformerSynced
	robotsSynced      cache.InformerSynced

	workQueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder
}

func NewController(
	kubeClientset kubernetes.Interface,
	robotClientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	robotInformer robotinformers.RobotInformer) *Controller {

	controller := &Controller{}

	return controller
}

func (c *Controller) Run(threadness int, stopCh <-chan struct{}) error {
	// wait for the caches to be synced before starting workers
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.robotsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	// launch threadness workers to process Robot resources
	for i := 0; i < threadness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workQueue.Done(obj)

		var (
			key string
			ok  bool
		)

		// key format: namespace/name
		// if obj is invalid, forget it
		if key, ok = obj.(string); !ok {
			c.workQueue.Forget(obj)
			return nil
		}

		// reconcile the actual state to the desired state
		if err := c.reconcile(key); err != nil {
			c.workQueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		// forget this item
		c.workQueue.Forget(obj)

		return nil

	}(obj)

	return err == nil
}

func (c *Controller) reconcile(key string) error {
	return nil
}
