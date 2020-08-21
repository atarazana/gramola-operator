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

// Gateway services names
const (
	GatewayServiceName     = "gateway"
	GatewayServicePort     = 8080
	GatewayServicePortName = "http"
	GatewayServiceImage    = "quay.io/cvicensa/gramola-gateway:0.0.2"
)

// GatewayServiceReplicas number of replicas for Gateway Service
var GatewayServiceReplicas = int32(2)

// NewGatewayDeploymentPatch returns a Patch
func NewGatewayDeploymentPatch(current *appsv1.Deployment) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	current.Spec.Replicas = &GatewayServiceReplicas
	current.Spec.Template.Spec.Containers[0].Image = GatewayServiceImage

	current.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/events",
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: GatewayServicePort,
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 25,
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
					IntVal: GatewayServicePort,
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		FailureThreshold:    3,
		InitialDelaySeconds: 27,
		PeriodSeconds:       2,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}

	return patch
}

// NewGatewayServicePatch returns a Patch
func NewGatewayServicePatch(current *corev1.Service) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}

// NewGatewayRoutePatch returns a Patch
func NewGatewayRoutePatch(current *routev1.Route) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}

// NewGatewayDeployment returns the deployment object for Gateway
func NewGatewayDeployment(instance *gramolav1.AppService, scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	annotations := GetGatewayAnnotations(instance)
	labels := GetAppServiceLabels(instance, GatewayServiceName)
	labels["app.kubernetes.io/name"] = "java"

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
			Name:        GatewayServiceName,
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
							Name:            GatewayServiceName,
							Image:           GatewayServiceImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          GatewayServicePortName,
									ContainerPort: GatewayServicePort,
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
										Path: "/api/events",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: GatewayServicePort,
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								FailureThreshold:    3,
								InitialDelaySeconds: 25,
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
											IntVal: GatewayServicePort,
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								FailureThreshold:    3,
								InitialDelaySeconds: 27,
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

// NewGatewayService return a Service object given name, namespace, etc.
func NewGatewayService(instance *gramolav1.AppService, scheme *runtime.Scheme) (*corev1.Service, error) {
	labels := GetAppServiceLabels(instance, GatewayServiceName)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     GatewayServicePortName,
					Port:     GatewayServicePort,
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

// NewGatewayRoute returns an OpenShift Route object
func NewGatewayRoute(instance *gramolav1.AppService, scheme *runtime.Scheme) (*routev1.Route, error) {
	labels := GetAppServiceLabels(instance, GatewayServiceName)
	targetPort := intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(GatewayServicePort),
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: GatewayServiceName,
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
