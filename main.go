package main

import (
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"robot-operator/pkg/controller"
	clientset "robot-operator/pkg/generated/clientset/versioned"
	robotinformers "robot-operator/pkg/generated/informers/externalversions"
	"robot-operator/pkg/signals"
)

const (
	threadness = 2
)

var (
	kubeConfig string
	masterURL  string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// setup signals
	stopCh := signals.SetupSignalHandler()

	kubeCfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeConfig)
	if err != nil {
		klog.Fatal("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		klog.Fatal("Error building kubernetes clientset: %s", err.Error())
	}

	robotClient, err := clientset.NewForConfig(kubeCfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	// create SharedInformerFactory
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	robotInformerFactory := robotinformers.NewSharedInformerFactory(robotClient, time.Second*30)

	// create Controller
	controller := controller.NewController(kubeClient, robotClient, kubeInformerFactory.Apps().V1().Deployments(), robotInformerFactory.Robot().V1().Robots())

	// start Informers
	kubeInformerFactory.Start(stopCh)
	robotInformerFactory.Start(stopCh)

	// run Controller
	if err := controller.Run(threadness, stopCh); err != nil {
		klog.Fatal("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeConfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
