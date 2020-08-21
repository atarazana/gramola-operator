package deployment

import (
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	version "github.com/atarazana/gramola-operator/version"

	gramolav1 "github.com/atarazana/gramola-operator/api/v1"
	// +kubebuilder:scaffold:imports
)

// NewIngress returns an OpenShift Route object
func NewIngress(cr *gramolav1.AppService, scheme *runtime.Scheme, name string, namespace string, host string, path string, serviceName string, servicePort int32) (*extensions.Ingress, error) {
	labels := GetAppServiceLabels(cr, name)
	annotations := map[string]string{
		"nginx.ingress.kubernetes.io/rewrite-target": "/$1",
	}
	servicePortIntOrString := intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(servicePort),
	}
	ingress := &extensions.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: extensions.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{
				{
					Host: host,
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path: path,
									Backend: extensions.IngressBackend{
										ServiceName: serviceName,
										ServicePort: servicePortIntOrString,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(cr, ingress, scheme); err != nil {
		return nil, err
	}

	return ingress, nil
}

// NewIngressPatch returns a Patch
func NewIngressPatch(current *extensions.Ingress) client.Patch {
	patch := client.MergeFrom(current.DeepCopy())

	current.Labels["version"] = version.Version

	return patch
}
