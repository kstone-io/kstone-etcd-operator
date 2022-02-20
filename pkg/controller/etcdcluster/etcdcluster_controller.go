/*
 * Tencent is pleased to support the open source community by making TKEStack
 * available.
 *
 * Copyright (C) 2012-2021 Tencent. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use
 * this file except in compliance with the License. You may obtain a copy of the
 * License at
 *
 * https://opensource.org/licenses/Apache-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OF ANY KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations under the License.
 */

package etcdcluster

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreInformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	klog "k8s.io/klog/v2"
	tAppV1 "tkestack.io/tapp/pkg/apis/tappcontroller/v1"
	tAppClientSet "tkestack.io/tapp/pkg/client/clientset/versioned"
	tAppInformer "tkestack.io/tapp/pkg/client/informers/externalversions/tappcontroller/v1"
	tAppLister "tkestack.io/tapp/pkg/client/listers/tappcontroller/v1"

	etcdv1alpha1 "tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
	etcdClientSet "tkestack.io/kstone-etcd-operator/pkg/client/clientset/versioned"
	etcdInformer "tkestack.io/kstone-etcd-operator/pkg/client/informers/externalversions/etcd/v1alpha1"
	etcdLister "tkestack.io/kstone-etcd-operator/pkg/client/listers/etcd/v1alpha1"
	"tkestack.io/kstone-etcd-operator/pkg/controller"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/metrics"
	// import service providers
	_ "tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/service/loadbalancer/providers"
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = etcdv1alpha1.SchemeGroupVersion.WithKind("EtcdCluster")

// Controller controls etcdclusters
type Controller struct {
	kubeClient clientset.Interface
	etcdClient etcdClientSet.Interface
	tAppClient tAppClientSet.Interface

	etcdLister   etcdLister.EtcdClusterLister
	tAppLister   tAppLister.TAppLister
	podLister    coreListers.PodLister
	svcLister    coreListers.ServiceLister
	secretLister coreListers.SecretLister
	cmLister     coreListers.ConfigMapLister

	etcdListerSynced   cache.InformerSynced
	tAppListerSynced   cache.InformerSynced
	podListerSynced    cache.InformerSynced
	svcListerSynced    cache.InformerSynced
	secretListerSynced cache.InformerSynced
	cmListerSynced     cache.InformerSynced

	syncHandler func(ctx context.Context, key string) error

	reconcileInterval       time.Duration
	statusReconcileInterval time.Duration

	clusters map[string]*Cluster

	recorder record.EventRecorder

	queue workqueue.RateLimitingInterface

	lock sync.Mutex
}

// NewEtcdClusterController create a new etcdcluster controller
func NewEtcdClusterController(
	reconcileInterval time.Duration,
	statusReconcileInterval time.Duration,
	etcdClient etcdClientSet.Interface,
	tAppClient tAppClientSet.Interface,
	kubeClient clientset.Interface,
	etcdInformer etcdInformer.EtcdClusterInformer,
	tAppInformer tAppInformer.TAppInformer,
	podInformer coreInformers.PodInformer,
	svcInformer coreInformers.ServiceInformer,
	secretInformer coreInformers.SecretInformer,
	cmInformer coreInformers.ConfigMapInformer,
) *Controller {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "etcdcluster-controller"})
	ec := &Controller{
		reconcileInterval:       reconcileInterval,
		statusReconcileInterval: statusReconcileInterval,
		etcdClient:              etcdClient,
		tAppClient:              tAppClient,
		kubeClient:              kubeClient,
		etcdLister:              etcdInformer.Lister(),
		tAppLister:              tAppInformer.Lister(),
		podLister:               podInformer.Lister(),
		svcLister:               svcInformer.Lister(),
		secretLister:            secretInformer.Lister(),
		cmLister:                cmInformer.Lister(),
		etcdListerSynced:        etcdInformer.Informer().HasSynced,
		tAppListerSynced:        tAppInformer.Informer().HasSynced,
		podListerSynced:         podInformer.Informer().HasSynced,
		svcListerSynced:         svcInformer.Informer().HasSynced,
		secretListerSynced:      secretInformer.Informer().HasSynced,
		cmListerSynced:          cmInformer.Informer().HasSynced,
		recorder:                recorder,
		queue:                   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "etcdcluster"),
		clusters:                make(map[string]*Cluster),
	}

	klog.Info("Setting up event handlers")

	etcdInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    ec.addEtcd,
		UpdateFunc: ec.updateEtcd,
		DeleteFunc: ec.deleteEtcd,
	}, 5*time.Minute)

	tAppInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ec.addTApp,
		UpdateFunc: ec.updateTApp,
		DeleteFunc: ec.deleteTApp,
	})

	svcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: ec.updateSvc,
	})

	ec.syncHandler = ec.syncEtcd

	metrics.Register()

	return ec
}

// UpdateEtcdCluster will create an etcd Cluster object to manage the specified etcd cluster,
// if the Cluster object exist, calling the function will trigger an update event to reconcile the etcd cluster
func (c *Controller) UpdateEtcdCluster(e *etcdv1alpha1.EtcdCluster) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.updateEtcdCluster(e)
}

func (c *Controller) updateEtcdCluster(e *etcdv1alpha1.EtcdCluster) {
	key := string(e.UID)

	if ec, ok := c.clusters[key]; ok {
		go ec.Update()
	} else {
		c.clusters[key] = NewCluster(
			e,
			c.reconcileInterval,
			c.statusReconcileInterval,
			c.recorder,
			c.kubeClient,
			c.etcdClient,
			c.tAppClient,
			c.etcdLister,
			c.tAppLister,
			c.podLister,
			c.svcLister,
			c.secretLister,
			c.cmLister,
		)

		go c.clusters[key].Run()
		go c.clusters[key].Update()

		if e.Status.Phase == etcdv1alpha1.ClusterCreating {
			metrics.EtcdStartLatency.Observe(time.Since(e.GetCreationTimestamp().Time).Seconds())
		}
	}
}

// StopControl will stop controlling the specified EtcdCluster
func (c *Controller) StopControl(e *etcdv1alpha1.EtcdCluster) {
	c.lock.Lock()
	defer c.lock.Unlock()

	key := string(e.UID)

	if mc, ok := c.clusters[key]; ok {
		mc.Stop()
		delete(c.clusters, key)
	}
}

func (c *Controller) addEtcd(obj interface{}) {
	etcd := obj.(*etcdv1alpha1.EtcdCluster)
	klog.V(4).InfoS("Adding etcdcluster", "etcdcluster", klog.KObj(etcd))
	if etcd.Status.Phase == etcdv1alpha1.ClusterCreating {
		metrics.EtcdObserveLatency.Observe(time.Since(etcd.GetCreationTimestamp().Time).Seconds())
	}
	c.enqueue(etcd)
}

func (c *Controller) updateEtcd(old, cur interface{}) {
	oldE := old.(*etcdv1alpha1.EtcdCluster)
	curE := cur.(*etcdv1alpha1.EtcdCluster)
	klog.V(4).InfoS("Updating etcdcluster", "etcdcluster", klog.KObj(oldE))
	c.enqueue(curE)
}

func (c *Controller) deleteEtcd(obj interface{}) {
	e, ok := obj.(*etcdv1alpha1.EtcdCluster)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		e, ok = tombstone.Obj.(*etcdv1alpha1.EtcdCluster)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a EtcdCluster %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Deleting etcdcluster", "etcdcluster", klog.KObj(e))
	metrics.ClustersTotal.Dec()
	c.StopControl(e)
}

func (c *Controller) addTApp(obj interface{}) {
	t := obj.(*tAppV1.TApp)
	if t.DeletionTimestamp != nil {
		c.deleteTApp(t)
		return
	}

	if controllerRef := metav1.GetControllerOf(t); controllerRef != nil {
		e := c.resolveControllerRef(t.Namespace, controllerRef)
		if e == nil {
			return
		}
		klog.V(4).InfoS("TApp added", "tapp", klog.KObj(t))
		c.enqueue(e)
		return
	}
	// we don't handle orphan tapp
}

func (c *Controller) updateTApp(old, cur interface{}) {
	oldT := old.(*tAppV1.TApp)
	curT := cur.(*tAppV1.TApp)
	if curT.ResourceVersion == oldT.ResourceVersion {
		return
	}

	oldControllerRef := metav1.GetControllerOf(oldT)
	curControllerRef := metav1.GetControllerOf(curT)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		if e := c.resolveControllerRef(oldT.Namespace, oldControllerRef); e != nil {
			c.enqueue(e)
		}
	}

	if curControllerRef != nil {
		e := c.resolveControllerRef(curT.Namespace, curControllerRef)
		if e == nil {
			return
		}
		klog.V(4).InfoS("TApp updated", "tapp", klog.KObj(curT))
		c.enqueue(e)
		return
	}
	// we don't handle orphan tapp
}

func (c *Controller) deleteTApp(obj interface{}) {
	t, ok := obj.(*tAppV1.TApp)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		t, ok = tombstone.Obj.(*tAppV1.TApp)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object than is not a TApp %#v", obj))
			return
		}
	}
	controllerRef := metav1.GetControllerOf(t)
	if controllerRef == nil {
		// No controller should care about orphans being deleted.
		return
	}
	e := c.resolveControllerRef(t.Namespace, controllerRef)
	if e == nil {
		return
	}
	klog.V(4).InfoS("TApp deleted", "tapp", klog.KObj(t))
	c.enqueue(e)
}

func (c *Controller) updateSvc(old, cur interface{}) {
	oldS := old.(*v1.Service)
	curS := cur.(*v1.Service)
	if curS.ResourceVersion == oldS.ResourceVersion {
		return
	}

	oldControllerRef := metav1.GetControllerOf(oldS)
	curControllerRef := metav1.GetControllerOf(curS)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		if e := c.resolveControllerRef(oldS.Namespace, oldControllerRef); e != nil {
			c.enqueue(e)
		}
	}

	if curControllerRef != nil {
		e := c.resolveControllerRef(curS.Namespace, curControllerRef)
		if e == nil {
			return
		}
		klog.V(4).InfoS("Service updated", "service", klog.KObj(curS))
		c.enqueue(e)
		return
	}
}

func (c *Controller) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) *etcdv1alpha1.EtcdCluster {
	if controllerRef.Kind != controllerKind.Kind {
		return nil
	}

	e, err := c.etcdLister.EtcdClusters(namespace).Get(controllerRef.Name)
	if err != nil {
		return nil
	}

	if e.UID != controllerRef.UID {
		return nil
	}
	return e
}

// Run runs the etcdcluster controller.
func (c *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting etcdcluster controller")
	defer klog.Infof("Shutting down etcdcluster controller")

	if !cache.WaitForNamedCacheSync("etcdcluster", ctx.Done(),
		c.etcdListerSynced, c.tAppListerSynced,
		c.podListerSynced, c.svcListerSynced,
		c.secretListerSynced, c.cmListerSynced,
	) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}
	<-ctx.Done()
}

func (c *Controller) worker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	if err := c.syncHandler(ctx, key.(string)); err != nil {
		utilruntime.HandleError(fmt.Errorf("error syncing EtcdCluster %v, requeuing: %v", key.(string), err))
		c.queue.AddRateLimited(key)
	} else {
		c.queue.Forget(key)
	}
	return true
}

func (c *Controller) syncEtcd(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing etcd cluster", "etcdcluster", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing etcd cluster", "etcdcluster", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	e, err := c.etcdLister.EtcdClusters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("EtcdCluster has been deleted", "etcdcluster", klog.KRef(namespace, name))
		return nil
	}
	if err != nil {
		return err
	}

	c.UpdateEtcdCluster(e.DeepCopy())

	return nil
}

func (c *Controller) enqueue(etcd *etcdv1alpha1.EtcdCluster) {
	key, err := controller.KeyFunc(etcd)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", etcd, err))
		return
	}

	c.queue.Add(key)
}

func init() {
	_ = etcdv1alpha1.AddToScheme(scheme.Scheme)
	_ = tAppV1.AddToScheme(scheme.Scheme)
}
