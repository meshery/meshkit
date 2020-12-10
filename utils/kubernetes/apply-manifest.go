package kubernetes

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type ApplyOptions struct {
	Namespace string
	Update    bool
	Delete    bool
}

// ApplyManifest applies, updates or deletes resources as specified in ApplyOptions.
// The namespace specified in ApplyOptions is used, if no namespace is specified then
// the namespace from manifest is used.
// If the the namespace does not exists, it will be created.
func (client *Client) ApplyManifest(contents []byte, recvOptions ApplyOptions) error {
	manifests := strings.Split(string(contents), "\n---\n")
	if len(manifests) > 0 && manifests[len(manifests)-1] == "\n" {
		manifests = manifests[:len(manifests)-1]
	}

	for _, manifest := range manifests {
		// create a fresh options var at each run
		options := recvOptions

		// decode YAML into unstructured.Unstructured
		obj := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		object, _, err := dec.Decode([]byte(manifest), nil, obj)
		if err != nil {
			if len(obj.GetObjectKind().GroupVersionKind().Kind) < 1 {
				continue
			}
			return ErrApplyManifest(err)
		}

		helper, err := constructObject(client.Clientset, client.RestConfig, object)
		if err != nil {
			return ErrApplyManifest(err)
		}

		// Default to namespace from the UI. If no namespace is passed, use the namespace used in the manifest.
		val, err := meta.NewAccessor().Namespace(object)
		if err == nil && len(val) > 1 {
			if len(options.Namespace) > 1 {
				er := meta.NewAccessor().SetNamespace(object, options.Namespace)
				if er != nil {
					return ErrApplyManifest(er)
				}
			} else {
				options.Namespace = val
			}
		}

		// Create namespace if it doesnt already exist
		if err = createNamespaceIfNotExist(context.TODO(), client.Clientset, options.Namespace); err != nil {
			return ErrApplyManifest(err)
		}

		if options.Delete {
			_, err = deleteObject(helper, options.Namespace, object)
			if err != nil && !kubeerror.IsNotFound(err) {
				return ErrApplyManifest(err)
			}
		} else {
			_, err = createObject(helper, options.Namespace, object, options.Update)
			if err != nil && !kubeerror.IsAlreadyExists(err) {
				return ErrApplyManifest(err)
			}
		}
	}

	return nil
}

func constructObject(kubeClientset kubernetes.Interface, restConfig rest.Config, obj runtime.Object) (*resource.Helper, error) {
	// Create a REST mapper that tracks information about the available resources in the cluster.
	groupResources, err := restmapper.GetAPIGroupResources(kubeClientset.Discovery())
	if err != nil {
		return nil, err
	}
	rm := restmapper.NewDiscoveryRESTMapper(groupResources)

	// Get some metadata needed to make the REST request.
	gvk := obj.GetObjectKind().GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return nil, err
	}

	// Create a client specifically for creating the object.
	restClient, err := newRestClient(restConfig, mapping.GroupVersionKind.GroupVersion())
	if err != nil {
		return nil, err
	}

	// Use the REST helper to create the object in the "default" namespace.
	return resource.NewHelper(restClient, mapping), nil
}

func newRestClient(restConfig rest.Config, gv schema.GroupVersion) (rest.Interface, error) {
	restConfig.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	restConfig.GroupVersion = &gv
	if len(gv.Group) == 0 {
		restConfig.APIPath = "/api"
	} else {
		restConfig.APIPath = "/apis"
	}

	return rest.RESTClientFor(&restConfig)
}

func createObject(restHelper *resource.Helper, namespace string, obj runtime.Object, update bool) (runtime.Object, error) {

	if update {
		_, _ = deleteObject(restHelper, namespace, obj)
	}

	object, err := restHelper.Create(namespace, update, obj)
	if err != nil {
		return nil, err
	}

	return object, nil
}

func deleteObject(restHelper *resource.Helper, namespace string, obj runtime.Object) (runtime.Object, error) {
	name, err := meta.NewAccessor().Name(obj)
	if err != nil {
		return nil, err
	}
	return restHelper.Delete(namespace, name)
}

func createNamespaceIfNotExist(ctx context.Context, kubeClientset kubernetes.Interface, namespace string) error {
	if namespace == "" {
		return nil
	}

	nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	_, err := kubeClientset.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
	if !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
