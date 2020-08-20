package deployment

import (
	gramolav1 "github.com/atarazana/gramola-operator/api/v1"
	// +kubebuilder:scaffold:imports
)

// Label Consts
const (
	AppName = "gramola"
)

// GetAppServiceLabels returns a map with the labels we want for all AppService assets
func GetAppServiceLabels(cr *gramolav1.AppService, component string) (labels map[string]string) {
	labels = map[string]string{
		"app":                         AppName,
		"component":                   component,
		"app.kubernetes.io/component": component,
		"app.kubernetes.io/instance":  component,
		"app.kubernetes.io/part-of":   AppName + "-app",
	}
	return labels
}
