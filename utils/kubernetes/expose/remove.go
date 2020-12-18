package expose

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Remove deletes the given service
func Remove(svc, ns string, client *kubernetes.Clientset) error {
	return client.CoreV1().Services(ns).Delete(context.TODO(), svc, metav1.DeleteOptions{})
}
