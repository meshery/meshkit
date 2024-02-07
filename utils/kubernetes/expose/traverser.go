package expose

import (
	"context"

	"fmt"

	"github.com/layer5io/meshkit/logger"
	"github.com/layer5io/meshkit/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// Traverser can be used to traverse resources in the cluster
type Traverser struct {
	Resources []Resource

	Client *kubernetes.Clientset

	Logger logger.Handler
}

// Resource defines the structure for the resource definition
// used by the traverser to locate the kubernetes resources
type Resource struct {
	Namespace string
	Type      string
	Name      string
}

// Object is the interface extending kubernetes' runtime
// object interface
type Object interface {
	runtime.Object
	GetNamespace() string
	GetName() string
}

// VisitCB is the type for callback function that is invoked
// when the traverser reaches one of the given kubernetes resource
//
// The first argument for the VisitCB is the kubernetes resource itself
// while the other is the error that may have occurred in the traverser
// in case this error is not nil, then this callbac function may return
// without executing the code
type VisitCB = func(Object, error) (*v1.Service, error)

// Visit function traverses each of the resource mentioned in the Traverser struct
func (traverser *Traverser) Visit(f VisitCB, continueOnError bool) ([]*v1.Service, error) {
	var errs []error
	var accumulatedSvcs []*v1.Service

	for _, res := range traverser.Resources {
		ns := res.Namespace // Namespace in which the resoruce should be searched
		typ := res.Type     // Type of the resource
		name := res.Name    // Name of the resource

		switch typ {
		case "Service":
			svc, err := traverser.Client.CoreV1().Services(ns).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			// Placing Kind and APIVersion manually
			// because client-go omits them
			// Please do no remove
			svc.Kind = typ
			svc.APIVersion = "v1"
			genSvc, err := f(svc, err)
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			if genSvc != nil {
				accumulatedSvcs = append(accumulatedSvcs, genSvc)
			}
		case "Pod":
			pod, err := traverser.Client.CoreV1().Pods(ns).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			// Placing Kind and APIVersion manually
			// because client-go omits them
			// Please do no remove
			pod.Kind = typ
			pod.APIVersion = "v1"
			genSvc, err := f(pod, err)
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			if genSvc != nil {
				accumulatedSvcs = append(accumulatedSvcs, genSvc)
			}
		case "ReplicationController":
			rc, err := traverser.Client.CoreV1().ReplicationControllers(ns).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			// Placing Kind and APIVersion manually
			// because client-go omits them
			// Please do no remove
			rc.Kind = typ
			rc.APIVersion = "v1"
			genSvc, err := f(rc, err)
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			if genSvc != nil {
				accumulatedSvcs = append(accumulatedSvcs, genSvc)
			}
		case "Deployment":
			dep, err := traverser.Client.AppsV1().Deployments(ns).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			// Placing Kind and APIVersion manually
			// because client-go omits them
			// Please do no remove
			dep.Kind = typ
			dep.APIVersion = "apps/v1"
			genSvc, err := f(dep, err)
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			if genSvc != nil {
				accumulatedSvcs = append(accumulatedSvcs, genSvc)
			}
		case "ReplicaSet":
			reps, err := traverser.Client.AppsV1().ReplicaSets(ns).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			// Placing Kind and APIVersion manually
			// because client-go omits them
			// Please do no remove
			reps.Kind = typ
			reps.APIVersion = "apps/v1"
			genSvc, err := f(reps, err)
			if err != nil {
				traverser.Logger.Error(err)
				errs = append(errs, err)
				if !continueOnError {
					return accumulatedSvcs, ErrGettingResource(err)
				}
			}
			if genSvc != nil {
				accumulatedSvcs = append(accumulatedSvcs, genSvc)
			}
		default:
			// Don't do anything
			traverser.Logger.Warn(fmt.Errorf("invalid resource type"))
		}
	}

	err := utils.CombineErrors(errs, "\n")
	if err != nil {
		return accumulatedSvcs, ErrTraverser(err)
	}

	return accumulatedSvcs, nil
}
