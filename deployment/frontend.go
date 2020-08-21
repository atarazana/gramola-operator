package deployment

import (
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
)

// Frontend services names
const (
	FrontendServiceName     = "frontend"
	FrontendServicePort     = 8080
	FrontendServicePortName = "http"
	FrontendServiceImage    = "quay.io/cvicensa/gramola-frontend:0.0.2"
)

// FrontendServiceReplicas number of replicas for Frontend Service
var FrontendServiceReplicas = int32(2)

// NewFrontendDeploymentPatch returns a Patch
func NewFrontendDeploymentPatch(current *appsv1.Deployment) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	current.Spec.Replicas = &FrontendServiceReplicas
	current.Spec.Template.Spec.Containers[0].Image = FrontendServiceImage

	current.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/health",
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: FrontendServicePort,
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		FailureThreshold:    5,
		InitialDelaySeconds: 26,
		PeriodSeconds:       2,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}

	current.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/health",
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: FrontendServicePort,
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 28,
		PeriodSeconds:       2,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}

	return patch
}

// NewFrontendServicePatch returns a Patch
func NewFrontendServicePatch(current *corev1.Service) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}

// NewFrontendRoutePatch returns a Patch
func NewFrontendRoutePatch(current *routev1.Route) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}

// NewFrontendDeployment returns the deployment object for Frontend
func NewFrontendDeployment(instance *gramolav1.AppService, scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	annotations := GetFrontendAnnotations(instance)
	labels := GetAppServiceLabels(instance, FrontendServiceName)
	labels["app.kubernetes.io/name"] = "nodejs"

	env := []corev1.EnvVar{
		{
			Name:  "NODE_ENV",
			Value: "production",
		},
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        FrontendServiceName,
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
							Name:            FrontendServiceName,
							Image:           FrontendServiceImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          FrontendServicePortName,
									ContainerPort: FrontendServicePort,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/health",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: FrontendServicePort,
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								FailureThreshold:    5,
								InitialDelaySeconds: 26,
								PeriodSeconds:       2,
								SuccessThreshold:    1,
								TimeoutSeconds:      1,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/health",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: FrontendServicePort,
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								FailureThreshold:    3,
								InitialDelaySeconds: 28,
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

// NewFrontendService return a Service object given name, namespace, etc.
func NewFrontendService(instance *gramolav1.AppService, scheme *runtime.Scheme) (*corev1.Service, error) {
	labels := GetAppServiceLabels(instance, FrontendServiceName)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      FrontendServiceName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     FrontendServicePortName,
					Port:     FrontendServicePort,
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

// NewFrontendRoute returns an OpenShift Route object
func NewFrontendRoute(instance *gramolav1.AppService, scheme *runtime.Scheme) (*routev1.Route, error) {
	labels := GetAppServiceLabels(instance, FrontendServiceName)
	targetPort := intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(FrontendServicePort),
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      FrontendServiceName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: FrontendServiceName,
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
