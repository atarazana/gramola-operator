package deployment

import (
	"os"
	"strconv"
	"strings"

	gramolav1 "github.com/atarazana/gramola-operator/api/v1"
	version "github.com/atarazana/gramola-operator/version"
	routev1 "github.com/openshift/api/route/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	intstr "k8s.io/apimachinery/pkg/util/intstr"

	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	util "github.com/atarazana/gramola-operator/util"
)

// Events services names
const (
	EventsServiceName          = "events"
	EventsServiceContainerName = "events"
	EventsServicePort          = 8080
	EventsServicePortName      = "http"
	EventsServiceImage         = "quay.io/cvicensa/gramola-events:0.0.2"

	EventsDatabaseServiceName          = EventsServiceName + "-database"
	EventsDatabaseServiceContainerName = "postgresql"
	EventsDatabaseServicePort          = 5432
	EventsDatabaseServicePortName      = "postgresql"
	EventsDatabaseServiceImage         = "registry.access.redhat.com/rhscl/postgresql-10-rhel7:latest"

	EventsDatabasePersistanceVolumeName      = EventsDatabaseServiceName + "-data"
	EventsDatabasePersistanceVolumeClaimName = EventsDatabaseServiceName
)

// Constants to locate the scripts to update the database
const (
	EventsDatabaseScriptsBaseEnvVarName = "DB_SCRIPTS_BASE_DIR"
	EventsDatabaseUpdateScriptName      = "events-database-update-0.0.2.sql"
	EventsDatabaseScriptsMountPath      = "/operator/scripts"

	EventsDatabaseCredentialsSecretName = EventsDatabaseServiceName
	EventsDatabaseScriptsConfigMapName  = EventsDatabaseServiceName + "-scripts"
)

// EventsDatabaseServiceReplicas number of replicas for Events Service
var EventsDatabaseServiceReplicas = int32(1)

// EventsServiceReplicas number of replicas for Events Service
var EventsServiceReplicas = int32(2)

// DatabaseCredentials contains the Database Credentials as a KV map
var DatabaseCredentials = map[string]string{
	"database-name":     "eventsdb",
	"database-password": "secret",
	"database-user":     "luke",
}

// DbScriptsBasePath point to the directory where the scripts to update the database should be
var DbScriptsBasePath = os.Getenv(EventsDatabaseScriptsBaseEnvVarName) + "/db"

// getDatabaseScriptsMap returns a KV map with script names as Ks and Script File names as Vs
func getDatabaseScriptsMap() map[string]string {
	scripts := make(map[string]string)
	databaseUser := DatabaseCredentials["database-user"]

	if dbUpdateScriptData, err := util.ReadFile(DbScriptsBasePath, EventsDatabaseUpdateScriptName); err == nil {
		dbUpdateScriptDataReplaced := strings.Replace(dbUpdateScriptData, "{{DB_USERNAME}}", databaseUser, -1)
		scripts[EventsDatabaseUpdateScriptName] = dbUpdateScriptDataReplaced
	}

	return scripts
}

// NewEventsDatabaseCredentialsSecret returns a Secret with the Events Database credentials
func NewEventsDatabaseCredentialsSecret(instance *gramolav1.AppService, scheme *runtime.Scheme) (*corev1.Secret, error) {
	labels := GetAppServiceLabels(instance, EventsDatabaseServiceName)

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      EventsDatabaseCredentialsSecretName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		StringData: DatabaseCredentials,
	}

	if err := controllerutil.SetControllerReference(instance, secret, scheme); err != nil {
		return nil, err
	}

	return secret, nil
}

// NewEventsDatabaseScriptsConfigMap returns a ConfigMap given a data object
func NewEventsDatabaseScriptsConfigMap(instance *gramolav1.AppService, scheme *runtime.Scheme) (*corev1.ConfigMap, error) {
	labels := GetAppServiceLabels(instance, EventsDatabaseServiceName)
	scripts := getDatabaseScriptsMap()

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      EventsDatabaseScriptsConfigMapName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Data: scripts,
	}

	if err := controllerutil.SetControllerReference(instance, configMap, scheme); err != nil {
		return nil, err
	}

	return configMap, nil
}

// NewEventsDatabaseScriptsConfigMapPatch returns a Patch
func NewEventsDatabaseScriptsConfigMapPatch(current *corev1.ConfigMap) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	scripts := getDatabaseScriptsMap()
	for k, v := range scripts {
		current.Data[k] = v
	}

	return patch
}

// NewEventsDatabaseCredentialsSecretPatch returns a Patch
func NewEventsDatabaseCredentialsSecretPatch(current *corev1.Secret) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	current.StringData = DatabaseCredentials

	return patch
}

// NewEventsDatabaseDeploymentPatch returns a Patch
func NewEventsDatabaseDeploymentPatch(current *appsv1.Deployment) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}

// NewEventsDeploymentPatch returns a Patch
func NewEventsDeploymentPatch(current *appsv1.Deployment) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	current.Spec.Replicas = &EventsServiceReplicas
	current.Spec.Template.Spec.Containers[0].Image = EventsServiceImage

	current.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/events",
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: int32(EventsServicePort),
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 20,
		PeriodSeconds:       2,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}

	current.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/events",
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: int32(EventsServicePort),
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 22,
		PeriodSeconds:       2,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}

	return patch
}

// NewEventsDatabaseServicePatch returns a Patch
func NewEventsDatabaseServicePatch(current *corev1.Service) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}

// NewEventsServicePatch returns a Patch
func NewEventsServicePatch(current *corev1.Service) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}

// NewEventsRoutePatch returns a Patch
func NewEventsRoutePatch(current *routev1.Route) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}

// NewEventsDeployment returns the deployment object for Events
func NewEventsDeployment(instance *gramolav1.AppService, scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	annotations := GetEventsAnnotations(instance)
	labels := GetAppServiceLabels(instance, EventsServiceName)
	labels["app.kubernetes.io/name"] = "java"

	env := []corev1.EnvVar{
		{
			Name: "DB_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "database-user",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: EventsDatabaseCredentialsSecretName,
					},
				},
			},
		},
		{
			Name: "DB_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "database-password",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: EventsDatabaseCredentialsSecretName,
					},
				},
			},
		},
		{
			Name: "DB_NAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "database-name",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: EventsDatabaseCredentialsSecretName,
					},
				},
			},
		},
		{
			Name:  "DB_SERVICE_NAME",
			Value: EventsDatabaseServiceName,
		},
		{
			Name:  "DB_SERVICE_PORT",
			Value: strconv.Itoa(EventsDatabaseServicePort),
		},
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        EventsServiceName,
			Namespace:   instance.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            EventsServiceContainerName,
							Image:           EventsServiceImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: EventsServicePort,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/events",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(EventsServicePort),
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								FailureThreshold:    3,
								InitialDelaySeconds: 20,
								PeriodSeconds:       2,
								SuccessThreshold:    1,
								TimeoutSeconds:      1,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/events",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(EventsServicePort),
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								FailureThreshold:    3,
								InitialDelaySeconds: 22,
								PeriodSeconds:       2,
								SuccessThreshold:    1,
								TimeoutSeconds:      1,
							},
							Env: env,
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(instance, deployment, scheme); err != nil {
		return nil, err
	}

	return deployment, nil
}

// NewEventsDatabaseDeployment returns the DB deployment for Events
func NewEventsDatabaseDeployment(instance *gramolav1.AppService, scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	labels := GetAppServiceLabels(instance, EventsDatabaseServiceName)
	labels["app.kubernetes.io/name"] = "postgresql"

	env := []corev1.EnvVar{
		{
			Name: "POSTGRESQL_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "database-user",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: EventsDatabaseCredentialsSecretName,
					},
				},
			},
		},
		{
			Name: "POSTGRESQL_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "database-password",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: EventsDatabaseCredentialsSecretName,
					},
				},
			},
		},
		{
			Name: "POSTGRESQL_DATABASE",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "database-name",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: EventsDatabaseCredentialsSecretName,
					},
				},
			},
		},
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      EventsDatabaseServiceName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &EventsDatabaseServiceReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            EventsDatabaseServiceContainerName,
							Image:           EventsDatabaseServiceImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          EventsDatabaseServicePortName,
									ContainerPort: EventsDatabaseServicePort,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/usr/libexec/check-container",
										},
									},
								},
								InitialDelaySeconds: 5,
								FailureThreshold:    3,
								TimeoutSeconds:      1,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/usr/libexec/check-container",
											"--live",
										},
									},
								},
								InitialDelaySeconds: 120,
								FailureThreshold:    3,
								TimeoutSeconds:      10,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      EventsDatabasePersistanceVolumeName,
									MountPath: "/var/lib/pgsql/data",
								},
								{
									Name:      EventsDatabaseScriptsConfigMapName,
									MountPath: EventsDatabaseScriptsMountPath,
								},
							},
							Env: env,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: EventsDatabasePersistanceVolumeName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: EventsDatabasePersistanceVolumeClaimName,
								},
							},
						},
						{
							Name: EventsDatabaseScriptsConfigMapName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: EventsDatabaseScriptsConfigMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(instance, deployment, scheme); err != nil {
		return nil, err
	}

	return deployment, nil
}

// NewEventsService return a Service object given name, namespace, etc.
func NewEventsService(instance *gramolav1.AppService, scheme *runtime.Scheme) (*corev1.Service, error) {
	labels := GetAppServiceLabels(instance, EventsServiceName)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      EventsServiceName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     EventsServicePortName,
					Port:     EventsServicePort,
					Protocol: "TCP",
				},
			},
			Selector: labels,
		},
	}

	if err := controllerutil.SetControllerReference(instance, service, scheme); err != nil {
		return nil, err
	}

	return service, nil
}

// NewEventsDatabaseService return a Service object given name, namespace, etc.
func NewEventsDatabaseService(instance *gramolav1.AppService, scheme *runtime.Scheme) (*corev1.Service, error) {
	labels := GetAppServiceLabels(instance, EventsDatabaseServiceName)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      EventsDatabaseServiceName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     EventsDatabaseServicePortName,
					Port:     EventsDatabaseServicePort,
					Protocol: "TCP",
				},
			},
			Selector: labels,
		},
	}

	if err := controllerutil.SetControllerReference(instance, service, scheme); err != nil {
		return nil, err
	}

	return service, nil
}

// NewEventsRoute returns an OpenShift Route object
func NewEventsRoute(instance *gramolav1.AppService, scheme *runtime.Scheme) (*routev1.Route, error) {
	labels := GetAppServiceLabels(instance, EventsServiceName)
	targetPort := intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(EventsServicePort),
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      EventsServiceName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: EventsServiceName,
			},
			Port: &routev1.RoutePort{
				TargetPort: targetPort,
			},
		},
	}

	if err := controllerutil.SetControllerReference(instance, route, scheme); err != nil {
		return nil, err
	}

	return route, nil
}
