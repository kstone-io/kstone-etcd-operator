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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.etcd.io/etcd/clientv3"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	tAppV1 "tkestack.io/tapp/pkg/apis/tappcontroller/v1"
	tAppClientSet "tkestack.io/tapp/pkg/client/clientset/versioned"
	tAppLister "tkestack.io/tapp/pkg/client/listers/tappcontroller/v1"

	"tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
	etcdv1alpha1 "tkestack.io/kstone-etcd-operator/pkg/apis/etcd/v1alpha1"
	etcdClientSet "tkestack.io/kstone-etcd-operator/pkg/client/clientset/versioned"
	etcdLister "tkestack.io/kstone-etcd-operator/pkg/client/listers/etcd/v1alpha1"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/constants"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/helpers"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/metrics"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/secure"
	"tkestack.io/kstone-etcd-operator/pkg/controller/etcdcluster/service"
	"tkestack.io/kstone-etcd-operator/pkg/utils"
)

// Cluster controls an etcd cluster's lifecycle
type Cluster struct {
	eventCh chan struct{}
	stopCh  chan struct{}

	etcd *v1alpha1.EtcdCluster

	kubeClient clientset.Interface
	etcdClient etcdClientSet.Interface
	tAppClient tAppClientSet.Interface

	etcdLister   etcdLister.EtcdClusterLister
	tAppLister   tAppLister.TAppLister
	podLister    coreListers.PodLister
	svcLister    coreListers.ServiceLister
	secretLister coreListers.SecretLister
	cmLister     coreListers.ConfigMapLister

	recorder record.EventRecorder

	reconcileInterval       time.Duration
	statusReconcileInterval time.Duration

	lastSync       time.Time
	lastStatusSync time.Time

	syncLock sync.Mutex
}

// NewCluster construct Cluster to control the etcd cluster's lifecycle
func NewCluster(
	etcd *v1alpha1.EtcdCluster,
	reconcileInterval time.Duration,
	statusReconcileInterval time.Duration,
	recorder record.EventRecorder,
	kubeClient clientset.Interface,
	etcdClient etcdClientSet.Interface,
	tAppClient tAppClientSet.Interface,
	etcdLister etcdLister.EtcdClusterLister,
	tAppLister tAppLister.TAppLister,
	podLister coreListers.PodLister,
	svcLister coreListers.ServiceLister,
	secretLister coreListers.SecretLister,
	cmLister coreListers.ConfigMapLister,
) *Cluster {
	return &Cluster{
		eventCh:                 make(chan struct{}, 1),
		stopCh:                  make(chan struct{}),
		etcd:                    etcd,
		reconcileInterval:       reconcileInterval,
		statusReconcileInterval: statusReconcileInterval,
		recorder:                recorder,
		kubeClient:              kubeClient,
		etcdClient:              etcdClient,
		tAppClient:              tAppClient,
		etcdLister:              etcdLister,
		tAppLister:              tAppLister,
		podLister:               podLister,
		svcLister:               svcLister,
		secretLister:            secretLister,
		cmLister:                cmLister,
		lastSync:                time.Now().Add(-1 * reconcileInterval),
		lastStatusSync:          time.Now().Add(-1 * statusReconcileInterval),
	}
}

// Run runs the Cluster controller.
func (c *Cluster) Run() {

	reconcileTicker := time.NewTicker(c.reconcileInterval)
	defer reconcileTicker.Stop()

	statusSyncTicker := time.NewTicker(c.statusReconcileInterval)
	defer statusSyncTicker.Stop()

	metrics.ClustersTotal.Inc()
	klog.Infof("run control process for etcd %s/%s", c.etcd.Namespace, c.etcd.Name)

	for {
		select {
		case <-c.stopCh:
			return
		case <-reconcileTicker.C:
			klog.Infof("start sync etcd %s/%s", c.etcd.Namespace, c.etcd.Name)
			metrics.EtcdSyncTotal.Inc()
			if err := c.sync(); err != nil {
				metrics.EtcdSyncFailedTotal.WithLabelValues(err.Error()).Inc()
				klog.Errorf("Sync etcd %s/%s failed, err: %s", c.etcd.Namespace, c.etcd.Name, err)
				c.recorder.Eventf(c.etcd, corev1.EventTypeWarning, "Sync Etcd Failed", "Sync etcd %s/%s failed, err: %s", c.etcd.Namespace, c.etcd.Name, err)
			} else {
				metrics.EtcdSyncSucceedTotal.Inc()
			}
		case <-statusSyncTicker.C:
			if err := c.syncStatus(); err != nil {
				klog.Error(err)
			}
		case <-c.eventCh:
			func() {
				klog.Infof("start sync etcd %s/%s", c.etcd.Namespace, c.etcd.Name)
				metrics.EtcdSyncTotal.Inc()
				if err := c.sync(); err != nil {
					metrics.EtcdSyncFailedTotal.WithLabelValues(err.Error()).Inc()
					klog.Errorf("Sync etcd %s/%s failed, err: %s", c.etcd.Namespace, c.etcd.Name, err)
					c.recorder.Eventf(c.etcd, corev1.EventTypeWarning, "Sync Etcd Failed", "Sync etcd %s/%s failed, err: %s", c.etcd.Namespace, c.etcd.Name, err)
				} else {
					metrics.EtcdSyncSucceedTotal.Inc()
				}
			}()
		}

		time.Sleep(time.Second * 2) // slow down
	}

}

func (c *Cluster) sync() error {
	c.syncLock.Lock()
	defer c.syncLock.Unlock()

	now := time.Now()

	defer func(start time.Time) {
		metrics.RecordEtcdSyncTimeCost(time.Since(start))
		metrics.RecordEtcdSyncTimeInterval(time.Since(c.lastSync))

		c.lastSync = time.Now()
	}(now)

	klog.Infof("sync cluster %s", c.etcd.Name)
	e, err := c.etcdLister.EtcdClusters(c.etcd.Namespace).Get(c.etcd.Name)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			return err
		}
		return nil
	}
	c.etcd = e

	if e.DeletionTimestamp != nil {
		// ensure all resources deleted
		return c.ensureEtcdDeleted()
	}

	// set status phase
	updated := e.DeepCopy()
	setEtcdStatusPhase(updated, etcdv1alpha1.ClusterCreating)
	if !reflect.DeepEqual(updated.Status, e.Status) {
		if e, err = c.etcdClient.EtcdV1alpha1().EtcdClusters(updated.Namespace).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{}); err != nil {
			return err
		}
		c.etcd = e
	}

	// ensure paused status
	if paused, err := c.ensurePausedStatus(); err != nil {
		return errors.Wrap(err, "ensure paused status failed")
	} else if paused {
		return nil
	}

	// ensure service and poll until load-balancer type service created
	if err := c.ensureService(); err != nil {
		return errors.Wrap(err, "ensure service failed")
	}

	// sync secret, create server and peer certificate, or sync server certificate SANs
	if err := c.ensureCert(); err != nil {
		return errors.Wrap(err, "ensure certificate failed")
	}

	// sync TApp: create a tapp to managed etcd pod
	// scale up and scale down
	if err := c.ensureTApp(); err != nil {
		return errors.Wrap(err, "ensure TApp failed")
	}

	// sync member status
	if err := c.ensureMemberStatus(); err != nil {
		return errors.Wrap(err, "ensure member status failed")
	}

	// sync learner
	if err := c.ensureLearner(); err != nil {
		return errors.Wrap(err, "ensure learner failed")
	}

	if err := c.syncStatus(); err != nil {
		return errors.Wrap(err, "sync status failed")
	}

	// sync etcd status: async, like tke cluster health check

	return nil
}

func (c *Cluster) ensureMemberStatus() error {
	e := c.etcd
	// must have a ready node
	tapp, err := c.tAppLister.TApps(e.Namespace).Get(helpers.NameForTApp(e))
	if err != nil {
		return err
	}
	if tapp.Status.ReadyReplicas < 1 {
		return nil
	}

	client, err := c.Client()
	if err != nil {
		return err
	}
	defer client.Close()
	m, err := client.MemberList(context.TODO())
	if err != nil {
		return err
	}

	hasLearner := false

	// update member
	memberStatus := make([]etcdv1alpha1.EtcdMember, 0, len(m.Members))
	for _, mm := range m.Members {
		found := false
		for _, esm := range e.Status.Members {
			if esm.ID == mm.ID {
				esm.Name = mm.Name
				esm.ClientURLs = mm.ClientURLs
				if mm.IsLearner {
					esm.Role = etcdv1alpha1.EtcdMemberLearner
					hasLearner = true
				}
				if mm.Name == "" {
					esm.Status = etcdv1alpha1.EtcdMemberStatusUnStarted
				} else {
					esm.Status = etcdv1alpha1.EtcdMemberStatusRunning
				}
				memberStatus = append(memberStatus, esm)
				found = true
			}
		}
		if !found {
			nm := etcdv1alpha1.EtcdMember{
				ID:         mm.ID,
				Name:       mm.Name,
				ClientURLs: mm.ClientURLs,
			}
			if mm.IsLearner {
				nm.Role = etcdv1alpha1.EtcdMemberLearner
				hasLearner = true
			}
			if nm.Name == "" {
				nm.Status = etcdv1alpha1.EtcdMemberStatusUnStarted
			} else {
				nm.Status = etcdv1alpha1.EtcdMemberStatusRunning
			}
			memberStatus = append(memberStatus, nm)
		}
	}
	// sort by name
	sort.Slice(memberStatus, func(i, j int) bool {
		if memberStatus[i].Name == "" || memberStatus[j].Name == "" {
			return memberStatus[j].Name == ""
		}
		return strings.Compare(memberStatus[i].Name, memberStatus[j].Name) < 0
	})

	// update member status
	for i := 0; i < len(memberStatus); i++ {
		if tapp.Status.Statuses[fmt.Sprint(i)] != tAppV1.InstanceRunning {
			continue
		}

		status, err := client.Status(context.TODO(), helpers.NodeAccessEndpoint(e, int32(i)))
		if err != nil {
			klog.Errorf("sync etcd %s/%s member status failed, index: %d, err: %v", e.Namespace, e.Name, i, err)
			continue
		}
		for j, ms := range memberStatus {
			if status.Header != nil && status.Header.MemberId == ms.ID {
				memberStatus[j].Version = status.Version
				if status.IsLearner {
					memberStatus[j].Role = etcdv1alpha1.EtcdMemberLearner
				} else if status.Leader == ms.ID {
					memberStatus[j].Role = etcdv1alpha1.EtcdMemberLeader
				} else {
					memberStatus[j].Role = etcdv1alpha1.EtcdMemberFollower
				}
			}
		}
	}

	if !reflect.DeepEqual(e.Status.Members, memberStatus) {
		ne := e.DeepCopy()
		ne.Status.Members = memberStatus
		_, err = c.etcdClient.EtcdV1alpha1().EtcdClusters(ne.Namespace).UpdateStatus(context.TODO(), ne, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		c.etcd = ne
	}

	// update tapp pod label if needed
	if len(e.Spec.Learners) == 0 && !hasLearner {
		newTApp := tapp.DeepCopy()
		for index, poTpl := range newTApp.Spec.TemplatePool {
			poTpl.Labels[constants.LabelEtcdVotingMemberSelector] = "true"
			newTApp.Spec.TemplatePool[index] = poTpl
		}
		if !reflect.DeepEqual(newTApp, tapp) {
			_, err = c.tAppClient.TappcontrollerV1().TApps(newTApp.Namespace).Update(context.TODO(), newTApp, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Cluster) ensureLearner() (err error) {
	e := c.etcd
	var client *clientv3.Client
	defer func() {
		if client != nil {
			client.Close()
		}
	}()
	for _, m := range e.Status.Members {
		if m.Role == etcdv1alpha1.EtcdMemberLearner && len(e.Spec.Learners) == 0 {
			client, err = c.Client()
			if err != nil {
				return err
			}
			ml, err := client.MemberList(context.TODO())
			if err != nil {
				return err
			}
			for _, mm := range ml.Members {
				if mm.ID == m.ID && mm.IsLearner {
					_, err = client.MemberPromote(context.TODO(), mm.ID)
					if err != nil {
						return err
					}
					// update tapp pod template, patch pod label
					return c.ensureMemberStatus()
				}
			}
		}
	}
	return nil
}

func (c *Cluster) ensurePausedStatus() (paused bool, err error) {
	if !helpers.CanOperate(c.etcd) {
		if !c.etcd.Status.ControlPaused {
			c.etcd.Status.ControlPaused = true
			_, err = c.etcdClient.EtcdV1alpha1().EtcdClusters(c.etcd.Namespace).UpdateStatus(context.TODO(), c.etcd, metav1.UpdateOptions{})
			return true, err
		}
		return true, nil
	}
	if c.etcd.Status.ControlPaused {
		c.etcd.Status.ControlPaused = false
		if _, err = c.etcdClient.EtcdV1alpha1().EtcdClusters(c.etcd.Namespace).UpdateStatus(context.TODO(), c.etcd, metav1.UpdateOptions{}); err != nil {
			return false, err
		}
	}
	return false, nil
}

// ensureService create a headless service for internal communication if not exist,
// create a ClusterIP or LoadBalancer type service for external access if not exist.
// using annotation for LoadBalancer provider
func (c *Cluster) ensureService() error {
	e := c.etcd

	internalSvc, err := service.NewEtcdHeadlessService(e)
	if err != nil {
		return err
	}

	// if internal service not existed, create it
	if _, err := c.svcLister.Services(internalSvc.Namespace).Get(internalSvc.Name); err != nil && k8serr.IsNotFound(err) {
		if _, err := c.kubeClient.CoreV1().Services(internalSvc.Namespace).Create(context.TODO(), internalSvc, metav1.CreateOptions{}); err != nil {
			return err
		}
		c.recorder.Eventf(e, corev1.EventTypeNormal, "HeadlessServiceCreated", "headless service for etcdcluster has been created")
	} else if err != nil {
		return err
	}

	externalSvc, err := service.NewEtcdAccessService(e)
	if err != nil {
		return err
	}
	if svc, err := c.svcLister.Services(externalSvc.Namespace).Get(externalSvc.Name); err != nil && k8serr.IsNotFound(err) {
		if _, err := c.kubeClient.CoreV1().Services(externalSvc.Namespace).Create(context.TODO(), externalSvc, metav1.CreateOptions{}); err != nil {
			return err
		}
		c.recorder.Eventf(e, corev1.EventTypeNormal, "AccessServiceCreated", "access service for etcdcluster has been created")
	} else if err != nil {
		return err
	} else {
		updated, err := service.GetUpdateAccessService(e, svc)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(updated, svc) {
			if _, err := c.kubeClient.CoreV1().Services(updated.Namespace).Update(context.TODO(), updated, metav1.UpdateOptions{}); err != nil {
				return err
			}
			c.recorder.Eventf(e, corev1.EventTypeNormal, "AccessServiceUpdated", "access service for etcdcluster has been updated")
		}
	}

	return c.ensureLoadBalancerStatus()
}

func (c *Cluster) ensureLoadBalancerStatus() error {
	// poll until load-balancer type service created and sync etcd load balancer
	return wait.PollImmediate(5*time.Second, 3*time.Minute, func() (done bool, err error) {
		e := c.etcd
		select {
		case <-c.stopCh:
			return false, errors.Errorf("etcd %s/%s cluster is stopped", e.Namespace, e.Name)
		default:
		}

		svc, err := c.svcLister.Services(e.Namespace).Get(helpers.NameForAccessService(e))
		if err != nil {
			return false, err
		}

		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			return true, nil
		}

		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return false, nil
		}

		if !reflect.DeepEqual(svc.Status.LoadBalancer, e.Status.LoadBalancer) {
			updated := e.DeepCopy()
			updated.Status.LoadBalancer = svc.Status.LoadBalancer
			if e, err = c.etcdClient.EtcdV1alpha1().EtcdClusters(e.Namespace).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{}); err != nil {
				return false, err
			}
			c.etcd = e
		}
		return true, nil
	})
}

// ensureCert ensure the certificate which a secure etcd cluster need existed, if not specify an existed cert,
// it will generate a self-signed CA, and using this CA to generate the server cert, peer cert, and the client cert
func (c *Cluster) ensureCert() error {
	e := c.etcd

	// skip sync if not specify auto generate certificate
	if e.Spec.Secure == nil || e.Spec.Secure.TLS == nil || e.Spec.Secure.TLS.AutoTLSCert == nil {
		// TODO: Delete Generated Certificate?
		return nil
	}

	autoTLSConfig := e.Spec.Secure.TLS.AutoTLSCert

	// ensure CA exist
	var caSecret *corev1.Secret
	if len(autoTLSConfig.ExternalCASecret) == 0 {
		defaultCAName := secure.NameForEtcdCertSecret(e.Name, secure.EtcdCACert)
		if existCA, err := c.secretLister.Secrets(e.Namespace).Get(defaultCAName); err != nil && k8serr.IsNotFound(err) {
			newCASecret, err := secure.NewEtcdCASecret(e)
			if err != nil {
				return err
			}
			if newCASecret, err = c.kubeClient.CoreV1().Secrets(newCASecret.Namespace).Create(context.TODO(), newCASecret, metav1.CreateOptions{}); err != nil {
				return err
			}
			c.recorder.Event(e, corev1.EventTypeNormal, "CACertCreated", "CA certificate secret has been created for etcdcluster")
			caSecret = newCASecret
		} else if err != nil {
			return err
		} else {
			caSecret = existCA
		}
	} else {
		existCA, err := c.secretLister.Secrets(e.Namespace).Get(autoTLSConfig.ExternalCASecret)
		if err != nil {
			c.recorder.Eventf(e, corev1.EventTypeWarning, "EnsureExternalCASecretFailed", "get external ca secret failed: %v", err)
			return err
		}
		caSecret = existCA
	}

	// TODO: update certificate

	for t, create := range map[secure.EtcdCertType]bool{
		secure.EtcdPeerCert:   autoTLSConfig.AutoGeneratePeerCert,
		secure.EtcdServerCert: autoTLSConfig.AutoGenerateServerCert,
		secure.EtcdClientCert: autoTLSConfig.AutoGenerateClientCert,
	} {
		if !create {
			continue
		}
		certName := secure.NameForEtcdCertSecret(e.Name, t)
		if _, err := c.secretLister.Secrets(e.Namespace).Get(certName); err != nil && k8serr.IsNotFound(err) {
			certSecret, err := secure.NewEtcdCertSecret(e, caSecret, t)
			if err != nil {
				return err
			}
			if _, err = c.kubeClient.CoreV1().Secrets(certSecret.Namespace).Create(context.TODO(), certSecret, metav1.CreateOptions{}); err != nil {
				return err
			}
			c.recorder.Eventf(e, corev1.EventTypeNormal, "CertCreated", "%s certificate secret has been created for etcdcluster", t)
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (c *Cluster) ensureTApp() error {
	e := c.etcd

	// new tapp template
	tapp, err := NewSeedTApp(e)
	if err != nil {
		return errors.Wrapf(err, "get tapp template failed")
	}
	// create if not exist
	existTApp, err := c.tAppLister.TApps(tapp.Namespace).Get(tapp.Name)
	if err != nil && k8serr.IsNotFound(err) {
		if _, err := c.tAppClient.TappcontrollerV1().TApps(tapp.Namespace).Create(context.TODO(), tapp, metav1.CreateOptions{}); err != nil {
			return err
		}
		c.recorder.Eventf(e, corev1.EventTypeNormal, "TAppCreated", "tapp for etcdcluster has been created")
		return nil
	} else if err != nil {
		return err
	}

	// support replace pv
	// support memory, local-pv
	// https to http
	return c.reconcileTApp(context.TODO(), existTApp)
}

func (c *Cluster) reconcileTApp(ctx context.Context, app *tAppV1.TApp) error {
	e := c.etcd
	// reconcile replicas: scale up or scale down
	// scale up
	if e.Spec.Size > app.Spec.Replicas {
		// wait existing pod running
		if app.Spec.Replicas != app.Status.Replicas || app.Status.ReadyReplicas != app.Status.Replicas {
			return nil
		}
		// ensure new member add
		client, err := c.Client()
		if err != nil {
			return err
		}
		defer client.Close()
		m, err := client.MemberList(ctx)
		if err != nil {
			return err
		}
		if len(m.Members) == int(app.Spec.Replicas) {
			if len(e.Spec.Learners) != 0 && e.Spec.Learners[0] == fmt.Sprint(app.Spec.Replicas) {
				klog.Infof("add learner for etcd %s/%s, index: %d", e.Namespace, e.Name, app.Spec.Replicas)
				if _, err = client.MemberAddAsLearner(ctx, []string{helpers.PeerNodeURL(e, app.Spec.Replicas)}); err != nil {
					return err
				}
			} else {
				klog.Infof("add member for etcd %s/%s, index: %d", e.Namespace, e.Name, app.Spec.Replicas)
				if _, err = client.MemberAdd(ctx, []string{helpers.PeerNodeURL(e, app.Spec.Replicas)}); err != nil {
					return err
				}
			}
		}
		// modify size and tapp template
		newApp := app.DeepCopy()
		newApp.Spec.Replicas++
		tpl := newApp.Spec.TemplatePool["0"]
		newTpl := *tpl.DeepCopy()
		if len(e.Spec.Learners) != 0 && e.Spec.Learners[0] == fmt.Sprint(app.Spec.Replicas) {
			newTpl.Labels[constants.LabelEtcdVotingMemberSelector] = "false"
		}
		newApp.Spec.TemplatePool[fmt.Sprint(app.Spec.Replicas)] = newTpl // the container args may be modified if not deepcopy
		newApp.Spec.Templates[fmt.Sprint(app.Spec.Replicas)] = fmt.Sprint(app.Spec.Replicas)
		ApplyEtcdNodeInitArgs(e, newApp)
		ApplyEtcdExtraArgs(e, newApp)
		if _, err = c.tAppClient.TappcontrollerV1().TApps(newApp.Namespace).Update(ctx, newApp, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	// scale down
	if e.Spec.Size < app.Spec.Replicas {
		// ensure member remove
		client, err := c.Client()
		if err != nil {
			return err
		}
		defer client.Close()
		m, err := client.MemberList(ctx)
		if err != nil {
			return err
		}

		var memberID uint64
		for _, mm := range m.Members {
			if utils.FindString(mm.PeerURLs, helpers.PeerNodeURL(e, app.Spec.Replicas-1)) {
				memberID = mm.ID
				break
			}
		}
		if memberID != 0 {
			if _, err = client.MemberRemove(ctx, memberID); err != nil {
				return err
			}
		}

		// modify size and tapp template
		newApp := app.DeepCopy()
		newApp.Spec.Replicas--
		delete(newApp.Spec.TemplatePool, fmt.Sprint(app.Spec.Replicas-1))
		delete(newApp.Spec.Templates, fmt.Sprint(app.Spec.Replicas-1))

		if _, err = c.tAppClient.TappcontrollerV1().TApps(newApp.Namespace).Update(ctx, newApp, metav1.UpdateOptions{}); err != nil {
			return err
		}

		// remove pvc
		pvc, err := c.kubeClient.CoreV1().PersistentVolumeClaims(newApp.Namespace).Get(ctx, helpers.PVCNameForEtcd(e, app.Spec.Replicas-1), metav1.GetOptions{})
		if err != nil && !k8serr.IsNotFound(err) {
			return err
		}
		if err == nil && metav1.IsControlledBy(pvc, newApp) {
			if err := c.kubeClient.CoreV1().PersistentVolumeClaims(newApp.Namespace).Delete(ctx, helpers.PVCNameForEtcd(e, app.Spec.Replicas-1), metav1.DeleteOptions{}); err != nil && !k8serr.IsNotFound(err) {
				return err
			}
		}
	}

	// update args, resource, affinity
	return c.updateTAppTemplate(app)
}

func (c *Cluster) updateTAppTemplate(app *tAppV1.TApp) error {
	e := c.etcd
	newApp := app.DeepCopy()

	for index, tpl := range newApp.Spec.TemplatePool {
		newTpl := tpl.DeepCopy()
		// affinity, toleration, topology
		newTpl.Spec.Affinity = e.Spec.Template.Affinity
		newTpl.Spec.Tolerations = e.Spec.Template.Tolerations
		newTpl.Spec.TopologySpreadConstraints = e.Spec.Template.TopologySpreadConstraints
		// resource
		newTpl.Spec.Containers[0].Resources = e.Spec.Template.Resources
		// env
		newTpl.Spec.Containers[0].Env = e.Spec.Template.Env

		newApp.Spec.TemplatePool[index] = *newTpl
	}

	// args
	ApplyEtcdNodeInitArgs(e, newApp)
	ApplyEtcdExtraArgs(e, newApp)

	if !reflect.DeepEqual(newApp, app) {
		if _, err := c.tAppClient.TappcontrollerV1().TApps(newApp.Namespace).Update(context.TODO(), newApp, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// ClientConfig return an etcd client config, for the operator to access etcd cluster
func (c *Cluster) ClientConfig() (*clientv3.Config, error) {
	e := c.etcd

	// ensure service exist
	_, err := c.svcLister.Services(e.Namespace).Get(helpers.NameForAccessService(e))
	if err != nil {
		return nil, err
	}

	cfg := &clientv3.Config{
		Endpoints:   []string{helpers.AccessURL(e)},
		DialTimeout: 15 * time.Second,
	}
	// tls config
	if helpers.IsClientSecure(e) {
		// get client cert
		var secret *corev1.Secret
		if e.Spec.Secure.TLS.ExternalCerts != nil && e.Spec.Secure.TLS.ExternalCerts.ClientSecret != "" {
			secret, err = c.secretLister.Secrets(e.Namespace).Get(e.Spec.Secure.TLS.ExternalCerts.ClientSecret)
		} else if e.Spec.Secure.TLS.AutoTLSCert != nil && e.Spec.Secure.TLS.AutoTLSCert.AutoGenerateClientCert {
			secret, err = c.secretLister.Secrets(e.Namespace).Get(secure.NameForEtcdCertSecret(e.Name, secure.EtcdClientCert))
		}
		if err != nil {
			return nil, err
		}

		tlsConfig, err := utils.NewTLSFromString(string(secret.Data[secure.EtcdCACertName]), string(secret.Data[secure.EtcdClientCertName]), string(secret.Data[secure.EtcdClientKeyName]))
		if err != nil {
			return nil, err
		}
		cfg.TLS = tlsConfig
	}

	// password authorization
	if e.Spec.Secure != nil && e.Spec.Secure.Auth != nil && e.Spec.Secure.Auth.CredentialSecretRef != nil {
		ref := e.Spec.Secure.Auth.CredentialSecretRef
		secret, err := c.secretLister.Secrets(ref.Namespace).Get(ref.Name)
		if err != nil {
			return nil, err
		}
		if cred := secure.CredentialFromSecret(secret); cred != nil {
			cfg.Username = cred.Username
			cfg.Password = cred.Password
		}
	}
	return cfg, nil
}

// Client return an etcd client which can be used to access the etcd cluster
func (c *Cluster) Client() (*clientv3.Client, error) {
	cfg, err := c.ClientConfig()
	if err != nil {
		return nil, err
	}
	return clientv3.New(*cfg)
}

// ensureEtcdDeleted ensure all etcd resources has been deleted,
// we use gc controller to clean resource now, so do nothing here.
func (c *Cluster) ensureEtcdDeleted() error {
	return nil
}

func (c *Cluster) syncStatus() error {
	now := time.Now()

	defer func(start time.Time) {
		metrics.RecordEtcdStatusSyncTimeInterval(time.Since(c.lastStatusSync))

		c.lastStatusSync = time.Now()
	}(now)

	if c.etcd == nil {
		return nil
	}

	e := c.etcd

	isCreating := c.etcd.Status.Phase == v1alpha1.ClusterCreating

	// set status phase
	tapp, err := c.tAppLister.TApps(e.Namespace).Get(helpers.NameForTApp(e))
	if err != nil {
		return err
	}

	if tapp.Status.ReadyReplicas == e.Spec.Size {
		updated := e.DeepCopy()
		setEtcdStatusPhase(updated, etcdv1alpha1.ClusterRunning)
		if !reflect.DeepEqual(updated.Status, e.Status) {
			if _, err = c.etcdClient.EtcdV1alpha1().EtcdClusters(updated.Namespace).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{}); err != nil {
				return err
			}
			if isCreating {
				metrics.RecordEtcdRunningLatency(time.Since(c.etcd.CreationTimestamp.Time))
			}
		}
	}

	return nil
}

// Update will trigger an update event that syncs the etcd cluster.
func (c *Cluster) Update() {
	select {
	case c.eventCh <- struct{}{}:
		klog.Infof("update event ch for etcd %s/%s", c.etcd.Namespace, c.etcd.Name)
	default:
	}
}

// Stop will stop the control of the etcd cluster.
func (c *Cluster) Stop() {
	close(c.stopCh)
}

func setEtcdStatusPhase(e *v1alpha1.EtcdCluster, nextPhase v1alpha1.EtcdClusterPhase) {
	phase := &e.Status.Phase
	switch nextPhase {
	case v1alpha1.ClusterPending:
	case v1alpha1.ClusterCreating:
		if *phase == v1alpha1.ClusterPending {
			*phase = v1alpha1.ClusterCreating
		}
	case v1alpha1.ClusterRunning:
		if *phase == v1alpha1.ClusterCreating || *phase == v1alpha1.ClusterUpdating || *phase == v1alpha1.ClusterUnHealthy {
			*phase = v1alpha1.ClusterRunning
		}
	case v1alpha1.ClusterUpdating:
		if *phase == v1alpha1.ClusterCreating || *phase == v1alpha1.ClusterRunning {
			*phase = v1alpha1.ClusterUpdating
		}
	case v1alpha1.ClusterFailed:
		*phase = v1alpha1.ClusterFailed
	case v1alpha1.ClusterUnHealthy:
		if *phase == v1alpha1.ClusterRunning {
			*phase = v1alpha1.ClusterUnHealthy
		}
	default:
		break
	}
}
