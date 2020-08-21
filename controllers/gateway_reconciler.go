package controllers

import (
	"context"
	"fmt"

	log "github.com/prometheus/common/log"

	errors "k8s.io/apimachinery/pkg/api/errors"
	types "k8s.io/apimachinery/pkg/types"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensions "k8s.io/api/extensions/v1beta1"

	_deployment "github.com/atarazana/gramola-operator/deployment"

	routev1 "github.com/openshift/api/route/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	gramolav1 "github.com/atarazana/gramola-operator/api/v1"
	// +kubebuilder:scaffold:imports
)

const (
	gatewayServiceName = "gateway"
)

// Reconciling Gateway
func (r *AppServiceReconciler) reconcileGateway(instance *gramolav1.AppService) (reconcile.Result, error) {

	if result, err := r.addGateway(instance); err != nil {
		return result, err
	}

	// Success
	return reconcile.Result{}, nil
}

func (r *AppServiceReconciler) addGateway(instance *gramolav1.AppService) (reconcile.Result, error) {
	if deployment, err := _deployment.NewGatewayDeployment(instance, r.Scheme); err == nil {
		if err := r.Client.Create(context.TODO(), deployment); err != nil {
			if errors.IsAlreadyExists(err) {
				from := &appsv1.Deployment{}
				if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, from); err == nil {
					patch := _deployment.NewGatewayDeploymentPatch(from)
					if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
						return reconcile.Result{}, err
					}
				}
			} else {
				return reconcile.Result{}, err
			}
		}
		// Events Database Deployment created/updated successfully
		log.Info(fmt.Sprintf("Created/Updated %s Deployment", deployment.Name))
		r.Recorder.Eventf(instance, "Normal", "Deployment Created/Updated", "Created/Updated %s Deployment", deployment.Name)
	} else {
		return reconcile.Result{}, err
	}

	if service, err := _deployment.NewGatewayService(instance, r.Scheme); err == nil {
		if err := r.Client.Create(context.TODO(), service); err != nil {
			if errors.IsAlreadyExists(err) {
				from := &corev1.Service{}
				if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, from); err == nil {
					patch := _deployment.NewGatewayServicePatch(from)
					if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
						return reconcile.Result{}, err
					}
				}
			} else {
				return reconcile.Result{}, err
			}
		}
		// Events Database Deployment created/updated successfully
		log.Info(fmt.Sprintf("Created/Updated %s Service", service.Name))
		r.Recorder.Eventf(instance, "Normal", "Service Created/Updated", "Created/Updated %s Service", service.Name)
	} else {
		return reconcile.Result{}, err
	}

	if instance.Spec.Platform == gramolav1.PlatformOpenShift {
		if route, err := _deployment.NewRoute(instance, r.Scheme, gatewayServiceName, instance.Namespace, gatewayServiceName, 8080); err == nil {
			if err := r.Client.Create(context.TODO(), route); err != nil {
				if errors.IsAlreadyExists(err) {
					from := &routev1.Route{}
					if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: route.Name, Namespace: route.Namespace}, from); err == nil {
						patch := _deployment.NewRoutePatch(from)
						if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
							return reconcile.Result{}, err
						}
					}
				} else {
					return reconcile.Result{}, err
				}
			}
			// Events Database Deployment created/updated successfully
			log.Info(fmt.Sprintf("Created/Updated %s Route", route.Name))
			r.Recorder.Eventf(instance, "Normal", "Route Created/Updated", "Created/Updated %s Route", route.Name)
		} else {
			return reconcile.Result{}, err
		}
	} else {
		gatewayHost := gatewayServiceName + "-" + instance.Namespace + "." + instance.Spec.DomainName
		if ingress, err := _deployment.NewIngress(instance, r.Scheme, gatewayServiceName, instance.Namespace, gatewayHost, "/(.*)", gatewayServiceName, 8080); err == nil {
			if err := r.Client.Create(context.TODO(), ingress); err != nil {
				if errors.IsAlreadyExists(err) {
					from := &extensions.Ingress{}
					if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, from); err == nil {
						patch := _deployment.NewIngressPatch(from)
						if err := r.Client.Patch(context.TODO(), from, patch); err != nil {
							return reconcile.Result{}, err
						}
					}
				} else {
					return reconcile.Result{}, err
				}
			}
			// Events Database Deployment created/updated successfully
			log.Info(fmt.Sprintf("Created/Updated %s Ingress", ingress.Name))
			r.Recorder.Eventf(instance, "Normal", "Ingress Created/Updated", "Created/Updated %s Ingress", ingress.Name)
		} else {
			return reconcile.Result{}, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}
