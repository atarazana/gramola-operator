package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/prometheus/common/log"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	_deployment "github.com/atarazana/gramola-operator/deployment"

	gramolav1 "github.com/atarazana/gramola-operator/api/v1"
	// +kubebuilder:scaffold:imports
)

// Events services names
const (
	EventsServiceName         = "events"
	EventsDatabaseServiceName = EventsServiceName + "-database"
)

// Constants to locate the scripts to update the database
const (
	DbScriptsBaseEnvVarName = "DB_SCRIPTS_BASE_DIR"
	DbUpdateScriptName      = "events-database-update-0.0.1.sql"
	DbScriptsMountPoint     = "/operator/scripts"
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
	databaseCredentials := map[string]string{
		"database-name":     "eventsdb",
		"database-password": "secret",
		"database-user":     "luke",
	}
	databaseSecret := _deployment.NewSecretFromStringData(instance, EventsDatabaseServiceName, instance.Namespace, databaseCredentials)
	if err := controllerutil.SetControllerReference(instance, databaseSecret, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.Client.Create(context.TODO(), databaseSecret); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Info(fmt.Sprintf("Created %s Secret", databaseSecret.Name))
		r.Recorder.Eventf(instance, "Normal", "Secret Created", "Created %s Secret", databaseSecret.Name)
	}

	scripts := make(map[string]string)
	if dbUpdateScriptData, err := readFile(DbUpdateScriptName); err == nil {
		dbUpdateScriptDataReplaced := strings.Replace(dbUpdateScriptData, "{{DB_USERNAME}}", databaseCredentials["database-user"], -1)
		scripts[DbUpdateScriptName] = dbUpdateScriptDataReplaced
	}

	//log.Info(fmt.Sprintf("scripts %s", scripts))

	databaseConfigMap := _deployment.NewConfigMapFromData(instance, EventsDatabaseServiceName+"-scripts", instance.Namespace, scripts)
	if err := controllerutil.SetControllerReference(instance, databaseConfigMap, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.Client.Create(context.TODO(), databaseConfigMap); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Info(fmt.Sprintf("Created %s ConfigMap", databaseConfigMap.Name))
		r.Recorder.Eventf(instance, "Normal", "ConfigMap Created", "Created %s ConfigMap", databaseConfigMap.Name)
	}

	databasePersistentVolumeClaim := _deployment.NewPersistentVolumeClaim(instance, EventsDatabaseServiceName, instance.Namespace, "512Mi")
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
	databaseDeployment := _deployment.NewEventsDatabaseDeployment(instance, EventsDatabaseServiceName, instance.Namespace, databaseSecret.Name, databaseConfigMap.Name, DbScriptsMountPoint)
	if err := controllerutil.SetControllerReference(instance, databaseDeployment, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.Client.Create(context.TODO(), databaseDeployment); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Info(fmt.Sprintf("Created %s Database", databaseDeployment.Name))
		r.Recorder.Eventf(instance, "Normal", "Deployment Created", "Created %s Database", databaseDeployment.Name)
	}

	databaseService := _deployment.NewService(instance, EventsDatabaseServiceName, instance.Namespace, []string{"postgresql"}, []int32{5432})
	if err := controllerutil.SetControllerReference(instance, databaseService, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.Client.Create(context.TODO(), databaseService); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Info(fmt.Sprintf("Created %s Service", databaseService.Name))
		r.Recorder.Eventf(instance, "Normal", "Service Created", "Created %s Service", databaseService.Name)
	}

	deployment := _deployment.NewEventsDeployment(instance, EventsServiceName, instance.Namespace, EventsDatabaseServiceName, EventsDatabaseServiceName, "5432")
	if err := controllerutil.SetControllerReference(instance, deployment, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.Client.Create(context.TODO(), deployment); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Info(fmt.Sprintf("Created %s Deployment", deployment.Name))
		r.Recorder.Eventf(instance, "Normal", "Deployment Created", "Created %s Deployment", deployment.Name)
	}

	service := _deployment.NewService(instance, EventsServiceName, instance.Namespace, []string{"http"}, []int32{8080})
	if err := controllerutil.SetControllerReference(instance, service, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.Client.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Info(fmt.Sprintf("Created %s Service", service.Name))
		r.Recorder.Eventf(instance, "Normal", "Service Created", "Created %s Service", service.Name)
	}

	if instance.Spec.Platform == gramolav1.PlatformOpenShift {
		route := _deployment.NewRoute(instance, EventsServiceName, instance.Namespace, EventsServiceName, 8080)
		if err := controllerutil.SetControllerReference(instance, route, r.Scheme); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.Client.Create(context.TODO(), route); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Info(fmt.Sprintf("Created %s Route", route.Name))
			r.Recorder.Eventf(instance, "Normal", "Route Created", "Created %s Route", route.Name)
		}
	}

	//Success
	return reconcile.Result{}, nil
}

func readFile(fileName string) (string, error) {
	filePath := DbScriptsBasePath + "/" + fileName
	log.Info(fmt.Sprintf("Reading file %s", fileName))
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("File reading error", err)
		return "", err
	}
	return string(data), nil
}
