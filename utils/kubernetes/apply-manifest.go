package kubernetes

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	v1 "k8s.io/api/core/v1"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type ApplyOptions struct {
	Namespace    string
	Update       bool
	Delete       bool
	IgnoreErrors bool
}

// ApplyManifest applies, updates or deletes resources as specified in ApplyOptions.
// The namespace specified in ApplyOptions is used, if no namespace is specified then
// the namespace from manifest is used.
// If the namespace does not exist, it is created when the credentials allow it.
// Credentials that cannot create namespaces are still allowed through, and the
// resource operation reports whether they can act inside the namespace.
func (client *Client) ApplyManifest(contents []byte, recvOptions ApplyOptions) error {
	manifests, err := splitManifests(contents)
	if err != nil {
		if recvOptions.IgnoreErrors {
			return nil
		}

		return ErrApplyManifest(err)
	}

	for _, manifest := range manifests {
		// create a fresh options var at each run
		options := recvOptions

		// get the runtime.Object
		object, _, err := GetObjectFromManifest(manifest)
		if err != nil {
			if recvOptions.IgnoreErrors || isEmptyDocument(manifest) {
				continue
			}

			return err
		}

		helper, err := constructObject(client.KubeClient, client.RestConfig, object)
		if err != nil {
			if recvOptions.IgnoreErrors {
				continue
			}

			return err
		}

		// Default to namespace from the UI. If no namespace is passed, use the namespace used in the manifest.
		val, err := meta.NewAccessor().Namespace(object)
		if err == nil && len(val) > 1 {
			if len(options.Namespace) > 1 {
				er := meta.NewAccessor().SetNamespace(object, options.Namespace)
				if er != nil {
					if recvOptions.IgnoreErrors {
						continue
					}

					return er
				}
			} else {
				options.Namespace = val
			}
		}

		// Create namespace if it doesnt already exist
		if err = createNamespaceIfNotExist(context.TODO(), client.KubeClient, options.Namespace); err != nil {
			if recvOptions.IgnoreErrors {
				continue
			}

			return err
		}

		if options.Delete {
			_, err = deleteObject(helper, options.Namespace, object)
			if err != nil && !kubeerror.IsNotFound(err) {
				if recvOptions.IgnoreErrors {
					continue
				}

				return err
			}
		} else {
			_, err = createObject(helper, options.Namespace, object, options.Update)
			if err != nil && !kubeerror.IsAlreadyExists(err) {
				if recvOptions.IgnoreErrors {
					continue
				}
				return err
			}
		}
	}

	return nil
}

// splitManifests splits a multi-document YAML stream on Kubernetes document
// separators. Splitting on the literal "\n---\n" missed a separator followed by
// a comment or trailing whitespace, and missed CRLF line endings entirely, so
// every document after the first was dropped without an error.
func splitManifests(contents []byte) ([]string, error) {
	reader := yamlutil.NewYAMLReader(bufio.NewReader(bytes.NewReader(contents)))

	var manifests []string
	for {
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return manifests, nil
		}
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, string(doc))
	}
}

// isEmptyDocument reports whether a document has nothing to apply: only
// whitespace, comments and document separators. Decoding one fails, and
// continuing past it is correct. Any other document that fails to decode is an
// error, because skipping it reports a deploy that never happened as a success.
func isEmptyDocument(manifest string) bool {
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "---"); ok {
			rest = strings.TrimSpace(rest)
			if rest == "" || strings.HasPrefix(rest, "#") {
				continue
			}
		}
		return false
	}
	return true
}

func GetObjectFromManifest(manifest string) (runtime.Object, *unstructured.Unstructured, error) {
	// decode YAML into unstructured.Unstructured
	obj := &unstructured.Unstructured{}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	object, _, err := dec.Decode([]byte(manifest), nil, obj)
	if err != nil {
		return nil, obj, ErrApplyManifest(err)
	}
	return object, obj, nil
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
	name, err := meta.NewAccessor().Name(obj)
	if err != nil {
		return nil, err
	}

	var object runtime.Object
	var er error

	object, err = restHelper.Create(namespace, update, obj)
	if err != nil {
		if kubeerror.IsAlreadyExists(err) && update {
			object, er = restHelper.Replace(namespace, name, update, obj)
			if er != nil {
				if (kubeerror.IsInvalid(er) && strings.Contains(er.Error(), "field is immutable")) || (kubeerror.IsInvalid(er) && strings.Contains(er.Error(), "primary clusterIP can not be unset")) {
					return object, nil
				}
				return nil, er
			}
			return object, nil
		}

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
	if err == nil || kubeerror.IsAlreadyExists(err) {
		return nil
	}
	// Namespace-scoped credentials cannot create namespaces at cluster scope,
	// even when the namespace exists and they can create resources inside it.
	// Let the resource operation that follows report the real error
	// instead of blocking every manifest on this pre-step.
	if kubeerror.IsForbidden(err) {
		return nil
	}
	return err
}
