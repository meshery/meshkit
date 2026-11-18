package kubernetes

import (
	"context"
	"errors"
	"testing"

	v1 "k8s.io/api/core/v1"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func namespaceCreateReactor(err error) k8stesting.ReactionFunc {
	return func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	}
}

// A credential confined to one namespace cannot create namespaces at cluster
// scope, so ApplyManifest must not fail before it gets to the manifest's
// namespaced resources. The fake clientset holds no namespace, so this also
// covers a namespace that genuinely does not exist: the pre-step stays quiet
// and the resource operation reports the NotFound naming it.
func TestCreateNamespaceIfNotExistToleratesForbidden(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "namespaces", namespaceCreateReactor(
		kubeerror.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "playground-sandbox", nil),
	))

	if err := createNamespaceIfNotExist(context.Background(), client, "playground-sandbox"); err != nil {
		t.Fatalf("createNamespaceIfNotExist() = %v, want nil for a namespace-scoped credential", err)
	}
}

// The pre-step predates this change and must still succeed quietly when the
// namespace is already there, whether the credential could create one or not.
func TestCreateNamespaceIfNotExistToleratesAlreadyExists(t *testing.T) {
	client := fake.NewSimpleClientset(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "playground-sandbox"}})

	if err := createNamespaceIfNotExist(context.Background(), client, "playground-sandbox"); err != nil {
		t.Fatalf("createNamespaceIfNotExist() = %v, want nil for an existing namespace", err)
	}
}

// Only the missing namespace permission is tolerated; every other API server
// error still has to reach the caller.
func TestCreateNamespaceIfNotExistReturnsOtherErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "namespaces", namespaceCreateReactor(
		kubeerror.NewInternalError(errors.New("apiserver unavailable")),
	))

	if err := createNamespaceIfNotExist(context.Background(), client, "playground-sandbox"); err == nil {
		t.Fatal("createNamespaceIfNotExist() = nil, want the API server error")
	}
}
