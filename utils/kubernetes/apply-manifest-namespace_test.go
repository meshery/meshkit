package kubernetes

import (
	"context"
	"errors"
	"testing"

	kubeerror "k8s.io/apimachinery/pkg/api/errors"
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
// namespaced resources.
func TestCreateNamespaceIfNotExistToleratesForbidden(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "namespaces", namespaceCreateReactor(
		kubeerror.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "playground-sandbox", nil),
	))

	if err := createNamespaceIfNotExist(context.Background(), client, "playground-sandbox"); err != nil {
		t.Fatalf("createNamespaceIfNotExist() = %v, want nil for a namespace-scoped credential", err)
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
