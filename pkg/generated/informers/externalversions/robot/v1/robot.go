/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	robotv1 "robot-operator/pkg/apis/robot/v1"
	versioned "robot-operator/pkg/generated/clientset/versioned"
	internalinterfaces "robot-operator/pkg/generated/informers/externalversions/internalinterfaces"
	v1 "robot-operator/pkg/generated/listers/robot/v1"
	time "time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// RobotInformer provides access to a shared informer and lister for
// Robots.
type RobotInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.RobotLister
}

type robotInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewRobotInformer constructs a new informer for Robot type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewRobotInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredRobotInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredRobotInformer constructs a new informer for Robot type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredRobotInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RobotV1().Robots(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RobotV1().Robots(namespace).Watch(context.TODO(), options)
			},
		},
		&robotv1.Robot{},
		resyncPeriod,
		indexers,
	)
}

func (f *robotInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredRobotInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *robotInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&robotv1.Robot{}, f.defaultInformer)
}

func (f *robotInformer) Lister() v1.RobotLister {
	return v1.NewRobotLister(f.Informer().GetIndexer())
}