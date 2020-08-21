package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/prometheus/common/log"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	_deployment "github.com/atarazana/gramola-operator/deployment"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	gramolav1 "github.com/atarazana/gramola-operator/api/v1"
	// +kubebuilder:scaffold:imports
)

// Constants to locate the scripts to update the database
const (
	DbScriptsBaseEnvVarName = "DB_SCRIPTS_BASE_DIR"
	//DbUpdateScriptName      = "events-database-update-0.0.2.sql"
	//DbScriptsMountPoint = "/operator/scripts"
)

// DbScriptsBasePath point to the directory where the scripts to update the database should be
var DbScriptsBasePath = os.Getenv(DbScriptsBaseEnvVarName) + "/db"

// Reconciling Events
func (r *AppServiceReconciler) reconcileEvents(instance *gramolav1.AppService) (reconcile.Result, error) {

	if result, err := r.addEvents(instance); err != nil {
		return result, err
	}

	// Success
	return reconcile.Result{}, nil
}

func (r *AppServiceReconciler) addEvents(instance *gramolav1.AppService) (reconcile.Result, error) {
	if databaseSecret, err := _deployment.NewEventsDatabaseCredentialsSecret(instance, r.Scheme); err == nil {
		if err := r.Client.Create(context.TODO(), databaseSecret); err != nil {
			if errors.IsAlreadyExists(err) {
				from := &corev1.Secret{}
				if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: databaseSecret.Name, Namespace: databaseSecret.Namespace}, from); err == nil {
					patch := _deployment.NewEventsDatabaseCredentialsSecretPatch(from)
					if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
						return reconcile.Result{}, err
					}
				}
			} else {
				return reconcile.Result{}, err
			}
		}
		// Secret created/updated successfully
		log.Info(fmt.Sprintf("Created/Updated %s Secret", databaseSecret.Name))
		r.Recorder.Eventf(instance, "Normal", "Secret Created/Updated", "Created/Updated %s Secret", databaseSecret.Name)
	} else {
		return reconcile.Result{}, err
	}

	// Create Events Database Script ConfigMap
	if databaseScriptsConfigMap, err := _deployment.NewEventsDatabaseScriptsConfigMap(instance, r.Scheme); err == nil {
		if err := r.Client.Create(context.TODO(), databaseScriptsConfigMap); err != nil {
			if errors.IsAlreadyExists(err) {
				from := &corev1.ConfigMap{}
				if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: databaseScriptsConfigMap.Name, Namespace: databaseScriptsConfigMap.Namespace}, from); err == nil {
					patch := _deployment.NewEventsDatabaseScriptsConfigMapPatch(from)
					if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
						return reconcile.Result{}, err
					}
				}
			} else {
				return reconcile.Result{}, err
			}
		}
		// ConfigMap created/updated successfully
		log.Info(fmt.Sprintf("Created/Updated %s ConfigMap", databaseScriptsConfigMap.Name))
		r.Recorder.Eventf(instance, "Normal", "ConfigMap Created/Updated", "Created/Updated %s ConfigMap", databaseScriptsConfigMap.Name)
	} else {
		return reconcile.Result{}, err
	}

	// PVC for Events Database
	databasePersistentVolumeClaim := _deployment.NewPersistentVolumeClaim(instance, _deployment.EventsDatabaseServiceName, instance.Namespace, "512Mi")
	if err := controllerutil.SetControllerReference(instance, databasePersistentVolumeClaim, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.Client.Create(context.TODO(), databasePersistentVolumeClaim); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Info(fmt.Sprintf("Created %s Persistent Volume Claim", databasePersistentVolumeClaim.Name))
		r.Recorder.Eventf(instance, "Normal", "PVC Created", "Created %s Persistent Volume Claim", databasePersistentVolumeClaim.Name)
	}

	// Adds environment variables from the secret values passed and also mounts a volume with the configmap also passed in
	if databaseDeployment, err := _deployment.NewEventsDatabaseDeployment(instance, r.Scheme); err == nil {
		if err := r.Client.Create(context.TODO(), databaseDeployment); err != nil {
			if errors.IsAlreadyExists(err) {
				from := &appsv1.Deployment{}
				if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: databaseDeployment.Name, Namespace: databaseDeployment.Namespace}, from); err == nil {
					patch := _deployment.NewEventsDatabaseDeploymentPatch(from)
					if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
						return reconcile.Result{}, err
					}
				}
			} else {
				return reconcile.Result{}, err
			}
		}
		// Events Database Deployment created/updated successfully
		log.Info(fmt.Sprintf("Created/Updated %s Deployment", databaseDeployment.Name))
		r.Recorder.Eventf(instance, "Normal", "Deployment Created/Updated", "Created/Updated %s Deployment", databaseDeployment.Name)
	} else {
		return reconcile.Result{}, err
	}

	if databaseService, err := _deployment.NewEventsDatabaseService(instance, r.Scheme); err == nil {
		if err := r.Client.Create(context.TODO(), databaseService); err != nil {
			if errors.IsAlreadyExists(err) {
				from := &corev1.Service{}
				if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: databaseService.Name, Namespace: databaseService.Namespace}, from); err == nil {
					patch := _deployment.NewEventsDatabaseServicePatch(from)
					if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
						return reconcile.Result{}, err
					}
				}
			} else {
				return reconcile.Result{}, err
			}
		}
		// Events Database Service created/updated successfully
		log.Info(fmt.Sprintf("Created/Updated %s Service", databaseService.Name))
		r.Recorder.Eventf(instance, "Normal", "Service Created/Updated", "Created/Updated %s Service", databaseService.Name)
	} else {
		return reconcile.Result{}, err
	}

	if eventsDeployment, err := _deployment.NewEventsDeployment(instance, r.Scheme); err == nil {
		if err := r.Client.Create(context.TODO(), eventsDeployment); err != nil {
			if errors.IsAlreadyExists(err) {
				from := &appsv1.Deployment{}
				if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: eventsDeployment.Name, Namespace: eventsDeployment.Namespace}, from); err == nil {
					patch := _deployment.NewEventsDeploymentPatch(from)
					if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
						return reconcile.Result{}, err
					}
				}
			} else {
				return reconcile.Result{}, err
			}
		}
		// Events Database Deployment created/updated successfully
		log.Info(fmt.Sprintf("Created/Updated %s Deployment", eventsDeployment.Name))
		r.Recorder.Eventf(instance, "Normal", "Deployment Created/Updated", "Created/Updated %s Deployment", eventsDeployment.Name)
	} else {
		return reconcile.Result{}, err
	}

	if eventsService, err := _deployment.NewEventsService(instance, r.Scheme); err == nil {
		if err := r.Client.Create(context.TODO(), eventsService); err != nil {
			if errors.IsAlreadyExists(err) {
				from := &corev1.Service{}
				if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: eventsService.Name, Namespace: eventsService.Namespace}, from); err == nil {
					patch := _deployment.NewEventsServicePatch(from)
					if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
						return reconcile.Result{}, err
					}
				}
			} else {
				return reconcile.Result{}, err
			}
		}
		// Events Database Deployment created/updated successfully
		log.Info(fmt.Sprintf("Created/Updated %s Service", eventsService.Name))
		r.Recorder.Eventf(instance, "Normal", "Service Created/Updated", "Created/Updated %s Service", eventsService.Name)
	} else {
		return reconcile.Result{}, err
	}

	//Success
	return reconcile.Result{}, nil
}

func readFile(fileName string) (string, error) {
	filePath := _deployment.DbScriptsBasePath + "/" + fileName
	log.Info(fmt.Sprintf("Reading file %s", fileName))
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("File reading error", err)
		return "", err
	}
	return string(data), nil
}
