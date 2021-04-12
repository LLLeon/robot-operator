package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslister "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	robotv1 "robot-operator/pkg/apis/robot/v1"
	clientset "robot-operator/pkg/generated/clientset/versioned"
	robotscheme "robot-operator/pkg/generated/clientset/versioned/scheme"
	robotinformers "robot-operator/pkg/generated/informers/externalversions/robot/v1"
	robotlisters "robot-operator/pkg/generated/listers/robot/v1"
)

const (
	controllerAgentName = "robot-operator"

	SuccessSynced         = "Synced"
	ErrResourceExists     = "ErrResourceExists"
	MessageResourceExists = "Resource %q already exists and is not managed by Robot"
	MessageResourceSynced = "Robot synced successfully"
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

	// Add robot-operator types to the default Kubernetes Scheme so Events can be
	// logged for robot-operator types.
	utilruntime.Must(robotscheme.AddToScheme(scheme.Scheme))

	// create event broadcaster
	klog.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeClientset:     kubeClientset,
		robotClientset:    robotClientset,
		deploymentsLister: deploymentInformer.Lister(),
		robotsLister:      robotInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		robotsSynced:      robotInformer.Informer().HasSynced,
		workQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Robots"),
		recorder:          recorder,
	}

	// set up an event handler for when Deployment resources change
	klog.Info("Setting up event handlers")
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			oldDepl := old.(*appsv1.Deployment)
			newDepl := new.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				return
			}

			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	// set up an event handler for when Robot resources change
	robotInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueRobot,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueRobot(new)
		},
	})

	return controller
}

func (c *Controller) Run(threadness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workQueue.ShutDown()

	klog.Info("Starting Robot controller")

	// wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.robotsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	// launch threadness workers to process Robot resources
	klog.Info("Starting workers")
	for i := 0; i < threadness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

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
		klog.Infof("Successfully synced '%s'", key)

		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) reconcile(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// here is the custom processing logic

	//  get the Robot resource with this namespace/name
	robot, err := c.robotsLister.Robots(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// get the deployment with the name specified in Robot.spec
	deploymentName := robot.Spec.DeploymentName
	if deploymentName == "" {
		utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}

	deployment, err := c.deploymentsLister.Deployments(robot.Namespace).Get(deploymentName)

	// if the resource doesn't exist, create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeClientset.AppsV1().Deployments(robot.Namespace).Create(context.TODO(), newDeployment(robot), metav1.CreateOptions{})
	}

	// requeue the item
	if err != nil {
		return err
	}

	// check whether deployment is controlled by robot
	if !metav1.IsControlledBy(deployment, robot) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(robot, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	if robot.Spec.Replicas != nil && *robot.Spec.Replicas != *deployment.Spec.Replicas {
		klog.V(4).Infof("Robot %s replicas: %d, Deployment %s replicas: %d", robot.Name, *robot.Spec.Replicas, deployment.Name, *deployment.Spec.Replicas)
		deployment, err = c.kubeClientset.AppsV1().Deployments(robot.Namespace).Update(context.TODO(), newDeployment(robot), metav1.UpdateOptions{})
	}

	// requeue the item
	if err != nil {
		return err
	}

	// update the status block of the Robot resource
	err = c.updateRobotStatus(robot, deployment)
	if err != nil {
		return err
	}

	c.recorder.Event(robot, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	return nil
}

func (c *Controller) handleObject(obj interface{}) {
	var (
		object metav1.Object
		ok     bool
	)

	//  obj should be any resource implementing metav1.Object
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Info("Recovered deleted object '%s' from tombstone", object.GetName())
	}

	klog.V(4).Info("Processing object: '%s'", object.GetName())

	// find the Robot resource that 'owns' the object
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != "Robot" {
			return
		}

		robot, err := c.robotsLister.Robots(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of foo '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		// then enqueues the Robot resource to be processed
		c.enqueueRobot(robot)
		return
	}
}

// enqueueRobot add the key of the object to workqueue.
func (c *Controller) enqueueRobot(obj interface{}) {
	var (
		key string
		err error
	)

	// generate key
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.workQueue.Add(key)
}

func (c *Controller) updateRobotStatus(robot *robotv1.Robot, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	robotCopy := robot.DeepCopy()
	robotCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	_, err := c.robotClientset.RobotV1().Robots(robot.Namespace).Update(context.TODO(), robotCopy, metav1.UpdateOptions{})

	return err
}

func newDeployment(robot *robotv1.Robot) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "nginx",
		"controller": robot.Name,
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      robot.Spec.DeploymentName,
			Namespace: robot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(robot, robotv1.SchemeGroupVersion.WithKind("Robot")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: robot.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
}
