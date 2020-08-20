package controllers

import (
	"context"
	"fmt"

	"github.com/prometheus/common/log"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	_deployment "github.com/atarazana/gramola-operator/deployment"

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
	deployment := _deployment.NewGatewayDeployment(instance, gatewayServiceName, instance.Namespace, []string{0: "events"})
	if err := controllerutil.SetControllerReference(instance, deployment, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.Client.Create(context.TODO(), deployment); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Info(fmt.Sprintf("Created %s Deployment", deployment.Name))
		r.Recorder.Eventf(instance, "Normal", "Deployment Created", "Created %s Deployment", deployment.Name)
	}

	service := _deployment.NewService(instance, gatewayServiceName, instance.Namespace, []string{"http"}, []int32{8080})
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
		route := _deployment.NewRoute(instance, gatewayServiceName, instance.Namespace, gatewayServiceName, 8080)
		if err := controllerutil.SetControllerReference(instance, route, r.Scheme); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.Client.Create(context.TODO(), route); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Info(fmt.Sprintf("Created %s Route", route.Name))
			r.Recorder.Eventf(instance, "Normal", "Route Created", "Created %s Route", route.Name)
		}
	} else {
		gatewayHost := gatewayServiceName + "-" + instance.Namespace + "." + instance.Spec.DomainName
		ingress := _deployment.NewIngress(instance, gatewayServiceName, instance.Namespace, gatewayHost, "/(.*)", gatewayServiceName, 8080)
		if err := controllerutil.SetControllerReference(instance, ingress, r.Scheme); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.Client.Create(context.TODO(), ingress); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Info(fmt.Sprintf("Created %s Ingress", ingress.Name))
			r.Recorder.Eventf(instance, "Normal", "Ingress Created", "Created %s Ingress", ingress.Name)
		}
	}

	//Success
	return reconcile.Result{}, nil
}
