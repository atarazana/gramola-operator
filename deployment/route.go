package deployment

import (
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	intstr "k8s.io/apimachinery/pkg/util/intstr"

	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	version "github.com/atarazana/gramola-operator/version"

	gramolav1 "github.com/atarazana/gramola-operator/api/v1"
	// +kubebuilder:scaffold:imports
)

// NewRoute returns an OpenShift Route object
func NewRoute(cr *gramolav1.AppService, scheme *runtime.Scheme, name string, namespace string, serviceName string, port int32) (*routev1.Route, error) {
	labels := GetAppServiceLabels(cr, name)
	targetPort := intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(port),
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			Port: &routev1.RoutePort{
				TargetPort: targetPort,
			},
		},
	}

	if err := controllerutil.SetControllerReference(cr, route, scheme); err != nil {
		return nil, err
	}

	return route, nil
}

// NewRoutePatch returns a Patch
func NewRoutePatch(current *routev1.Route) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}
