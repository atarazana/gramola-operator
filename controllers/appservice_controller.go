/*


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

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	gramolav1 "github.com/atarazana/gramola-operator/api/v1"
	_deployment "github.com/atarazana/gramola-operator/deployment"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Best practices
const controllerName = "controller-appservice"

const (
	errorAlias                    = "Not a proper AppService object because Alias is not valid"
	errorPlatform                 = "Not a proper AppService object because Platform is not valid"
	errorDomainName               = "DomainName is not valid"
	errorNotAppServiceObject      = "Not a AppService object"
	errorAppServiceObjectNotValid = "Not a valid AppService object"
	errorUnableToUpdateInstance   = "Unable to update instance"
	errorUnableToUpdateStatus     = "Unable to update status"
	errorUnexpected               = "Unexpected error"
)

//var log = logf.Log.WithName(controllerName)

// AppServiceReconciler reconciles a AppService object
type AppServiceReconciler struct {
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	// Best practices...
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=gramola.atarazana.com,resources=appservices,verbs=*
// +kubebuilder:rbac:groups=gramola.atarazana.com,resources=appservices/finalizers,verbs=*
// +kubebuilder:rbac:groups=gramola.atarazana.com,resources=appservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods;pods/exec;services;services/finalizers;endpoints;persistentvolumeclaims;events;configmaps;secrets;serviceaccounts,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list
// +kubebuilder:rbac:groups=apps,resources=deployments;deployments/finalizers;daemonsets;replicasets;statefulsets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create
// +kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=*
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=*

// Reconcile reads that state of the cluster for a AppService object and makes changes based on the state read
// and what is in the AppService.Spec
func (r *AppServiceReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	//ctx := context.Background()
	log := r.Log.WithValues("appservice", request.NamespacedName)
	log.Info("Reconciling AppService")

	// Fetch the AppService instance
	instance := &gramolav1.AppService{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	log.Info(fmt.Sprintf("Status %s", instance.Status))

	// Validate the CR instance
	if ok, err := r.isValid(instance); !ok {
		return r.ManageError(instance, err)
	}

	// Now that we have a target let's initialize the CR instance. Updates Spec.Initilized in `instance`
	if initialized, err := r.isInitialized(instance); err == nil && !initialized {
		err := r.Client.Update(context.TODO(), instance)
		if err != nil {
			log.Error(err, errorUnableToUpdateInstance, "instance", instance)
			return r.ManageError(instance, err)
		}
		return reconcile.Result{}, nil
	} else {
		if err != nil {
			return r.ManageError(instance, err)
		}
	}

	//////////////////////////
	// Events
	//////////////////////////
	if _, err := r.reconcileEvents(instance); err != nil {
		return r.ManageError(instance, err)
	}

	//////////////////////////
	// Gateway
	//////////////////////////
	if _, err := r.reconcileGateway(instance); err != nil {
		return r.ManageError(instance, err)
	}

	//////////////////////////
	// Frontend
	//////////////////////////
	if _, err := r.reconcileFrontend(instance); err != nil {
		return r.ManageError(instance, err)
	}

	//////////////////////////
	// Update Events DataBase
	//////////////////////////
	// Update only if not applied before with success
	if !r.CurrentDatabaseScriptWasRun(instance) {
		// TODO Backup DB

		// Start the Script Run
		scriptRun := &gramolav1.DatabaseScriptRun{
			Script: _deployment.EventsDatabaseUpdateScriptName,
			Status: gramolav1.DatabaseUpdateStatusUnknown,
		}
		if dataBaseUpdated, err := r.UpdateEventsDatabase(request); err != nil {
			log.Error(err, "Error DB update", "instance", instance)
			// Update Status
			scriptRun.Status = gramolav1.DatabaseUpdateStatusFailed
			instance.Status.EventsDatabaseScriptRuns = append(instance.Status.EventsDatabaseScriptRuns, *scriptRun)
			instance.Status.EventsDatabaseUpdated = gramolav1.DatabaseUpdateStatusFailed
			return r.ManageError(instance, err)
		} else {
			if dataBaseUpdated {
				log.Info(fmt.Sprintf("dataBaseUpdated ====> %s", instance.Status))
				// Update Status
				scriptRun.Status = gramolav1.DatabaseUpdateStatusSucceeded
				instance.Status.EventsDatabaseScriptRuns = append(instance.Status.EventsDatabaseScriptRuns, *scriptRun)
				instance.Status.EventsDatabaseUpdated = gramolav1.DatabaseUpdateStatusSucceeded
			} else {
				// Maybe the Database Pods weren't ready but running... so scchedule a new reconcile cycle
				return r.ManageSuccess(instance, 10*time.Second, gramolav1.RequeueEvent)
			}
		}
	}

	// Nothing else to do
	return r.ManageSuccess(instance, 0, gramolav1.NoAction)
}

// isValid checks if our CR is valid or not
func (r *AppServiceReconciler) isValid(obj metav1.Object) (bool, error) {
	//log.Info(fmt.Sprintf("isValid? %s", obj))

	instance, ok := obj.(*gramolav1.AppService)
	if !ok {
		err := k8s_errors.NewBadRequest(errorNotAppServiceObject)
		log.Error(err, errorNotAppServiceObject)
		return false, err
	}

	// Check Alias
	if len(instance.Spec.Alias) > 0 &&
		instance.Spec.Alias != "Gramola" && instance.Spec.Alias != "Gramophone" && instance.Spec.Alias != "Phonograph" {
		err := k8s_errors.NewBadRequest(errorAlias)
		log.Error(err, errorAlias)
		return false, err
	}

	// Check Platform
	if len(instance.Spec.Platform) > 0 && instance.Spec.Platform != gramolav1.PlatformKubernetes && instance.Spec.Platform != gramolav1.PlatformOpenShift {
		err := k8s_errors.NewBadRequest(errorPlatform)
		log.Error(err, errorPlatform)
		return false, err
	}

	// Check DomainName if platform is kubernetes
	if instance.Spec.Platform == gramolav1.PlatformKubernetes {
		if matched, err := regexp.MatchString(gramolav1.DomainNameRegex, instance.Spec.DomainName); !matched || err != nil {
			err := k8s_errors.NewBadRequest(errorDomainName)
			log.Error(err, errorDomainName)
			return false, err
		}
	}

	return true, nil
}

// IsInitialized checks if our CR has been initialized or not
func (r *AppServiceReconciler) isInitialized(obj metav1.Object) (bool, error) {
	instance, ok := obj.(*gramolav1.AppService)
	if !ok {
		err := k8s_errors.NewBadRequest(errorNotAppServiceObject)
		log.Error(err, errorNotAppServiceObject)
		return false, err
	}
	if instance.Spec.Initialized {
		return true, nil
	}

	// initilize Platform
	if len(instance.Spec.Platform) <= 0 {
		instance.Spec.Platform = gramolav1.PlatformKubernetes
	}

	if instance.Spec.Platform == gramolav1.PlatformKubernetes && len(instance.Spec.DomainName) <= 0 {
		instance.Spec.DomainName = gramolav1.DefaultDomainName
	}

	// Set as Initilized
	// TODO add a Finalizer...
	// util.AddFinalizer(mycrd, controllerName)
	instance.Spec.Initialized = true
	return false, nil
}

// ManageError manages an error object, an instance of the CR is passed along
func (r *AppServiceReconciler) ManageError(obj metav1.Object, issue error) (reconcile.Result, error) {
	log.Error(issue, "Error managed")
	runtimeObj, ok := (obj).(runtime.Object)
	if !ok {
		err := k8s_errors.NewBadRequest("not a runtime.Object")
		log.Error(err, "passed object was not a runtime.Object", "object", obj)
		r.Recorder.Event(runtimeObj, "Error", "ProcessingError", err.Error())
		return reconcile.Result{}, nil
	}
	var retryInterval time.Duration
	r.Recorder.Event(runtimeObj, "Warning", "ProcessingError", issue.Error())
	if instance, ok := (obj).(*gramolav1.AppService); ok {
		lastUpdate := instance.Status.LastUpdate
		lastStatus := instance.Status.Status
		status := gramolav1.ReconcileStatus{
			LastUpdate: metav1.Now(),
			Reason:     issue.Error(),
			Status:     gramolav1.AppServiceConditionStatusFailed,
		}
		instance.Status.ReconcileStatus = status
		instance.Status.LastAction = gramolav1.NoAction
		err := r.Client.Status().Update(context.Background(), runtimeObj)
		if err != nil {
			log.Error(err, errorUnableToUpdateStatus)
			return reconcile.Result{
				RequeueAfter: time.Second,
				Requeue:      true,
			}, nil
		}
		if lastUpdate.IsZero() || lastStatus == "Success" {
			retryInterval = time.Second
		} else {
			retryInterval = status.LastUpdate.Sub(lastUpdate.Time.Round(time.Second))
		}
	} else {
		log.Info("object is not RecocileStatusAware, not setting status")
		retryInterval = time.Second
	}
	return reconcile.Result{
		RequeueAfter: time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Hour.Nanoseconds()*6))),
		Requeue:      true,
	}, nil
}

// ManageSuccess manages a success and updates status accordingly, an instance of the CR is passed along
func (r *AppServiceReconciler) ManageSuccess(obj metav1.Object, requeueAfter time.Duration, action gramolav1.ActionType) (reconcile.Result, error) {
	log.Info(fmt.Sprintf("===> ManageSuccess with requeueAfter: %d from: %s", requeueAfter, action))
	runtimeObj, ok := (obj).(runtime.Object)
	if !ok {
		log.Error(k8s_errors.NewBadRequest("not a runtime.Object"), "passed object was not a runtime.Object", "object", obj)
		return reconcile.Result{}, nil
	}
	if instance, ok := (obj).(*gramolav1.AppService); ok {
		status := gramolav1.ReconcileStatus{
			LastUpdate: metav1.Now(),
			Reason:     "",
			Status:     gramolav1.AppServiceConditionStatusTrue,
		}
		instance.Status.ReconcileStatus = status
		if len(action) <= 0 {
			instance.Status.LastAction = gramolav1.NoAction
		} else {
			instance.Status.LastAction = action
		}

		err := r.Client.Status().Update(context.Background(), runtimeObj)
		if err != nil {
			log.Error(err, "Unable to update status")
			r.Recorder.Event(runtimeObj, "Warning", "ProcessingError", "Unable to update status")
			return reconcile.Result{
				RequeueAfter: time.Second,
				Requeue:      true,
			}, nil
		}
		//if instance.Status.IsCanaryRunning {
		//	r.Recorder.Event(runtimeObj, "Normal", "StatusUpdate", fmt.Sprintf("AppService in progress %d%%", instance.Status.CanaryWeight))
		//}
	} else {
		log.Info("object is not AppService, not setting status")
		r.Recorder.Event(runtimeObj, "Warning", "ProcessingError", "Object is not AppService, not setting status")
	}

	if requeueAfter > 0 {
		return reconcile.Result{
			RequeueAfter: requeueAfter,
			Requeue:      true,
		}, nil
	}
	return reconcile.Result{}, nil
}

// UpdateEventsDatabase runs a script in the first 'Events' database pod found (and ready) returns true if the script was run succesfully
func (r *AppServiceReconciler) UpdateEventsDatabase(request reconcile.Request) (bool, error) {
	// List all pods of the Events Database
	podList := &corev1.PodList{}
	lbs := map[string]string{
		"component": _deployment.EventsDatabaseServiceName,
	}
	labelSelector := labels.SelectorFromSet(lbs)
	listOps := &client.ListOptions{Namespace: request.Namespace, LabelSelector: labelSelector}
	if err := r.Client.List(context.TODO(), podList, listOps); err != nil {
		return false, err
	}

	// Count the pods that are pending or running as available
	var ready []corev1.Pod
	for _, pod := range podList.Items {
		log.Info(fmt.Sprintf("pod: %s phase: %s statuses: %v", pod.Name, pod.Status.Phase, pod.Status.ContainerStatuses))
		if pod.Status.Phase == corev1.PodRunning {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.Name == _deployment.EventsDatabaseServiceContainerName && containerStatus.Ready {
					ready = append(ready, pod)
					break
				}
			}
		}
	}

	log.Info(fmt.Sprintf("ready: %v", ready))

	if len(ready) > 0 {
		filePath := _deployment.EventsDatabaseScriptsMountPath + "/" + _deployment.EventsDatabaseUpdateScriptName
		if _out, _err, err := r.ExecuteRemoteCommand(&ready[0], "psql -U $POSTGRESQL_USER $POSTGRESQL_DATABASE -f "+filePath); err != nil {
			return false, err
		} else {
			log.Info(fmt.Sprintf("stdout: %s\nstderr: %s", _out, _err))
			if len(_err) > 0 {
				return false, errors.Wrapf(err, "Failed executing script %s on %s", filePath, _deployment.EventsDatabaseServiceName)
			} else {
				errorFound := regexp.MustCompile(`(?i)error`)
				if errorFound.MatchString(_out) {
					return false, errors.Wrapf(err, "Failed executing script %s on %s", filePath, _deployment.EventsDatabaseServiceName)
				}
				return true, nil
			}
		}
	}

	return false, nil
}

// ExecuteRemoteCommand executes a remote shell command on the given pod
// returns the output from stdout and stderr
func (r *AppServiceReconciler) ExecuteRemoteCommand(pod *corev1.Pod, command string) (string, string, error) {
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	restCfg, err := kubeCfg.ClientConfig()
	if err != nil {
		return "", "", err
	}
	coreClient, err := corev1client.NewForConfig(restCfg)
	if err != nil {
		return "", "", err
	}

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	request := coreClient.RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"/bin/bash", "-c", command},
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(restCfg, "POST", request.URL())
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", errors.Wrapf(err, "Failed executing command %s on %v/%v", command, pod.Namespace, pod.Name)
	}

	return buf.String(), errBuf.String(), nil
}

// CurrentDatabaseScriptWasRun checks if the current Database Update Script was run
func (r *AppServiceReconciler) CurrentDatabaseScriptWasRun(instance *gramolav1.AppService) bool {
	for i := range instance.Status.EventsDatabaseScriptRuns {
		if instance.Status.EventsDatabaseScriptRuns[i].Script == _deployment.EventsDatabaseUpdateScriptName &&
			instance.Status.EventsDatabaseScriptRuns[i].Status == gramolav1.DatabaseUpdateStatusSucceeded {
			return true
		}
	}

	return false
}

// Predicate to manage all events as a composite predicate
func appServicePredicateComposite() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			log.Info("AppService (predicate->CreateEvent) " + e.Meta.GetName())
			if _, ok := e.Object.(*gramolav1.AppService); ok {
				return appServicePredicate().Create(e)
			}
			if _, ok := e.Object.(*corev1.Pod); ok {
				return podPredicate().Create(e)
			}

			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			log.Info("AppService (predicate->UpdateEvent) " + e.MetaNew.GetName())
			if _, ok := e.ObjectNew.(*gramolav1.AppService); ok {
				return appServicePredicate().Update(e)
			}
			if _, ok := e.ObjectNew.(*corev1.Pod); ok {
				returned := podPredicate().Update(e)
				log.Info(fmt.Sprintf("OJO %t", returned))
				return true
			}

			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log.Info("AppService (predicate->DeleteEvent) " + e.Meta.GetName())
			if _, ok := e.Object.(*gramolav1.AppService); ok {
				return appServicePredicate().Delete(e)
			}
			if _, ok := e.Object.(*corev1.Pod); ok {
				return podPredicate().Delete(e)
			}

			return false
		},
	}
}

// Predicate to manage events from AppServices
func appServicePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			log.Info("AppService (predicate->UpdateEvent) " + e.MetaNew.GetName())
			// Check that new and old objects are the expected type
			_, ok := e.ObjectOld.(*gramolav1.AppService)
			if !ok {
				log.Error(nil, "Update event has no old proper runtime object to update", "event", e)
				return false
			}
			newServiceConfig, ok := e.ObjectNew.(*gramolav1.AppService)
			if !ok {
				log.Error(nil, "Update event has no proper new runtime object for update", "event", e)
				return false
			}
			if !newServiceConfig.Spec.Enabled {
				log.Error(nil, "Runtime object is not enabled", "event", e)
				return false
			}

			// Also check if no change in ResourceGeneration to return false
			if e.MetaOld == nil {
				log.Error(nil, "Update event has no old metadata", "event", e)
				return false
			}
			if e.MetaNew == nil {
				log.Error(nil, "Update event has no new metadata", "event", e)
				return false
			}
			if e.MetaNew.GetGeneration() == e.MetaOld.GetGeneration() {
				return false
			}

			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			log.Info("AppService (predicate->CreateFunc) " + e.Meta.GetName())
			if _, ok := e.Object.(*gramolav1.AppService); !ok {
				return false
			}

			return true
		},
	}
}

// Predicate to manage events from Pods
func podPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			log.Info("Pod (predicate->UpdateEvent) " + e.MetaNew.GetName())

			// Ignore if not events-database-*
			if !strings.Contains(e.MetaNew.GetName(), "events-database") {
				log.Info("Pod is not events-database - [IGNORED]")
				return false
			}

			// Check that new and old objects are the expected type
			_, ok := e.ObjectOld.(*corev1.Pod)
			if !ok {
				log.Error(nil, "Update event has no old proper runtime object to update", "event", e)
				return false
			}
			newPod, ok := e.ObjectNew.(*corev1.Pod)
			if !ok {
				log.Error(nil, "Update event has no proper new runtime object for update", "event", e)
				return false
			}
			log.Info(fmt.Sprintf("Pod (events-database) Status Phase %s ready for patch? %t", newPod.Status.Phase, (newPod.Status.Phase == corev1.PodRunning || newPod.Status.Phase == corev1.PodSucceeded)))
			if newPod.Status.Phase != corev1.PodRunning && newPod.Status.Phase != corev1.PodSucceeded {
				log.Info("Pod is not Running - [IGNORED]", "event", e.MetaNew.GetName())
				return false
			}

			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			log.Info("Pod (predicate->CreateFunc) " + e.Meta.GetName() + "- [IGNORED]")
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log.Info("Pod (predicate->DeleteEvent) " + e.Meta.GetName() + "- [IGNORED]")
			return false
		},
	}
}

// SetupWithManager is called from main.go
func (r *AppServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&gramolav1.AppService{}).
		WithEventFilter(appServicePredicate()).
		Build(r)
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &gramolav1.AppService{},
	}, podPredicate())
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &gramolav1.AppService{},
	})
	if err != nil {
		return err
	}

	return nil
}

// SetupWithManagerOld is called from main.go
func (r *AppServiceReconciler) SetupWithManagerOld(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gramolav1.AppService{}).
		WithEventFilter(appServicePredicateComposite()).
		Owns(&corev1.Pod{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
