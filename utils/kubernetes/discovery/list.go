package discovery

// import (
// 	"context"
// 	"time"

// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
// )

// // ListNamespaces will list namespace items
// func (c *Client) ListLocalResource(ctx context.Context, config Config, opts metav1.ListOptions) (result *unstructured.UnstructuredList, err error) {
// 	var timeout time.Duration
// 	if opts.TimeoutSeconds != nil {
// 		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
// 	}
// 	result = &unstructured.UnstructuredList{}
// 	err = c.restClient.Get().
// 		Namespace(config.Namespace).
// 		Resource(config.Resource).
// 		VersionedParams(&opts, config.Codec).
// 		Timeout(timeout).
// 		Do(ctx).
// 		Into(result)
// 	return
// }

// func (c *Client) ListGlobalResource(ctx context.Context, config Config, opts metav1.ListOptions) (result *unstructured.UnstructuredList, err error) {

// 	var Scheme = runtime.NewScheme()
// 	var Codecs = serializer.NewCodecFactory(Scheme)
// 	var ParameterCodec = runtime.NewParameterCodec(Scheme)

// 	var timeout time.Duration
// 	if opts.TimeoutSeconds != nil {
// 		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
// 	}
// 	result = &unstructured.UnstructuredList{}
// 	err = c.restClient.Get().
// 		Resource(config.Resource).
// 		VersionedParams(&opts, config.Codec).
// 		Timeout(timeout).
// 		Do(ctx).
// 		Into(result)
// 	return
// }
