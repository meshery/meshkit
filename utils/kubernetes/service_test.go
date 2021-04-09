package kubernetes

import (
	"context"
	"reflect"
	"testing"

	"github.com/layer5io/meshkit/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetServiceEndpoint(t *testing.T) {
	type args struct {
		ctx    context.Context
		client kubernetes.Interface
		opts   *ServiceOptions
	}

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test_service",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "1.1.1.1",
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name:     "test_port_1",
					Port:     1000,
					NodePort: 2000,
				},
				corev1.ServicePort{
					Name:     "test_port_2",
					Port:     1001,
					NodePort: 2001,
				},
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					corev1.LoadBalancerIngress{
						IP:       "10.10.10.10",
						Hostname: "test_name.com",
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		args    args
		want    *utils.Endpoint
		wantErr bool
	}{
		{
			name: "service with desired namespace",
			args: args{
				ctx:    context.TODO(),
				client: fake.NewSimpleClientset(svc),
				opts: &ServiceOptions{
					Name:         "test_service",
					Namespace:    "default",
					PortSelector: "test_port_2",
					APIServerURL: "https://20.20.20.20:8443",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "10.10.10.10:1001",
					},
				},
			},
			want: &utils.Endpoint{
				External: &utils.HostPort{
					Address: "10.10.10.10",
					Port:    1001,
				},
				Internal: &utils.HostPort{
					Address: "1.1.1.1",
					Port:    1001,
				},
			},
			wantErr: false,
		},
		{
			name: "service with undesired name",
			args: args{
				ctx:    context.TODO(),
				client: fake.NewSimpleClientset(svc),
				opts: &ServiceOptions{
					Name:         "non_test_service",
					Namespace:    "default",
					PortSelector: "test_port_2",
					APIServerURL: "https://20.20.20.20:8443",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "10.10.10.10:1001",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service with undesired namespace",
			args: args{
				ctx:    context.TODO(),
				client: fake.NewSimpleClientset(svc),
				opts: &ServiceOptions{
					Name:         "non_test_service",
					Namespace:    "non_default",
					PortSelector: "test_port_2",
					APIServerURL: "https://20.20.20.20:8443",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "10.10.10.10:1001",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetServiceEndpoint(tt.args.ctx, tt.args.client, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetServiceEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetServiceEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEndpoint(t *testing.T) {
	type args struct {
		ctx  context.Context
		opts *ServiceOptions
		obj  *corev1.Service
	}

	tests := []struct {
		name    string
		args    args
		want    *utils.Endpoint
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "with PortSelector",
			args: args{
				ctx: context.TODO(),
				opts: &ServiceOptions{
					PortSelector: "test_port_1",
					APIServerURL: "https://20.20.20.20:8443",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "10.10.10.10:1000",
					},
				},
				obj: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test_service",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "1.1.1.1",
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name:     "test_port_1",
								Port:     1000,
								NodePort: 2000,
							},
							corev1.ServicePort{
								Name:     "test_port_2",
								Port:     1001,
								NodePort: 2001,
							},
						},
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								corev1.LoadBalancerIngress{
									IP:       "10.10.10.10",
									Hostname: "test_name.com",
								},
							},
						},
					},
				},
			},
			want: &utils.Endpoint{
				External: &utils.HostPort{
					Address: "10.10.10.10",
					Port:    1000,
				},
				Internal: &utils.HostPort{
					Address: "1.1.1.1",
					Port:    1000,
				},
			},
			wantErr: false,
		},
		{
			name: "without PortSelector",
			args: args{
				ctx: context.TODO(),
				opts: &ServiceOptions{
					APIServerURL: "https://20.20.20.20:8443",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "10.10.10.10:1001",
					},
				},
				obj: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test_service",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "1.1.1.1",
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name:     "test_port_1",
								Port:     1000,
								NodePort: 2000,
							},
							corev1.ServicePort{
								Name:     "test_port_2",
								Port:     1001,
								NodePort: 2001,
							},
						},
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								corev1.LoadBalancerIngress{
									IP:       "10.10.10.10",
									Hostname: "test_name.com",
								},
							},
						},
					},
				},
			},
			want: &utils.Endpoint{
				External: &utils.HostPort{
					Address: "10.10.10.10",
					Port:    1001,
				},
				Internal: &utils.HostPort{
					Address: "1.1.1.1",
					Port:    1001,
				},
			},
			wantErr: false,
		},
		{
			name: "minikube with Invalid APIServer URL",
			args: args{
				ctx: context.TODO(),
				opts: &ServiceOptions{
					PortSelector: "test_port_1",
					//APIServerURL: "https://20.20.20.20:8443",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "20.20.20.20:2000",
					},
				},
				obj: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test_service",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "1.1.1.1",
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name:     "test_port_1",
								Port:     1000,
								NodePort: 2000,
							},
							corev1.ServicePort{
								Name:     "test_port_2",
								Port:     1001,
								NodePort: 2001,
							},
						},
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								corev1.LoadBalancerIngress{
									IP:       "10.10.10.10",
									Hostname: "test_name.com",
								},
							},
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "minikube with valid APIServer URL",
			args: args{
				ctx: context.TODO(),
				opts: &ServiceOptions{
					PortSelector: "test_port_1",
					APIServerURL: "https://20.20.20.20:8443",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "20.20.20.20:2000",
					},
				},
				obj: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test_service",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "1.1.1.1",
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name:     "test_port_1",
								Port:     1000,
								NodePort: 2000,
							},
							corev1.ServicePort{
								Name:     "test_port_2",
								Port:     1001,
								NodePort: 2001,
							},
						},
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								corev1.LoadBalancerIngress{
									IP:       "10.10.10.10",
									Hostname: "test_name.com",
								},
							},
						},
					},
				},
			},
			want: &utils.Endpoint{
				External: &utils.HostPort{
					Address: "20.20.20.20",
					Port:    2000,
				},
				Internal: &utils.HostPort{
					Address: "1.1.1.1",
					Port:    1000,
				},
			},
			wantErr: false,
		},
		{
			name: "service type NodePort",
			args: args{
				ctx: context.TODO(),
				opts: &ServiceOptions{
					PortSelector: "test_port_2",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "localhost:2001",
					},
				},
				obj: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test_service",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "1.1.1.1",
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name:     "test_port_1",
								Port:     1000,
								NodePort: 2000,
							},
							corev1.ServicePort{
								Name:     "test_port_2",
								Port:     1001,
								NodePort: 2001,
							},
						},
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{},
				},
			},
			want: &utils.Endpoint{
				External: &utils.HostPort{
					Address: "localhost",
					Port:    2001,
				},
				Internal: &utils.HostPort{
					Address: "1.1.1.1",
					Port:    1001,
				},
			},
			wantErr: false,
		},
		{
			name: "service type LoadBalancer with host instead of IP Address",
			args: args{
				ctx: context.TODO(),
				opts: &ServiceOptions{
					PortSelector: "test_port_2",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "test_name.com:1001",
					},
				},
				obj: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test_service",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "1.1.1.1",
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name:     "test_port_1",
								Port:     1000,
								NodePort: 2000,
							},
							corev1.ServicePort{
								Name:     "test_port_2",
								Port:     1001,
								NodePort: 2001,
							},
						},
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								corev1.LoadBalancerIngress{
									Hostname: "test_name.com",
								},
							},
						},
					},
				},
			},
			want: &utils.Endpoint{
				External: &utils.HostPort{
					Address: "test_name.com",
					Port:    1001,
				},
				Internal: &utils.HostPort{
					Address: "1.1.1.1",
					Port:    1001,
				},
			},
			wantErr: false,
		},
		{
			name: "service type ClusterIP",
			args: args{
				ctx: context.TODO(),
				opts: &ServiceOptions{
					PortSelector: "test_port_2",
					Mock: &utils.MockOptions{
						DesiredEndpoint: "1.1.1.1:1001",
					},
				},
				obj: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test_service",
						Namespace:   "default",
						Annotations: map[string]string{},
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "1.1.1.1",
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name: "test_port_1",
								Port: 1000,
							},
							corev1.ServicePort{
								Name: "test_port_2",
								Port: 1001,
							},
						},
						Type: corev1.ServiceTypeClusterIP,
					},
				},
			},
			want: &utils.Endpoint{
				Internal: &utils.HostPort{
					Address: "1.1.1.1",
					Port:    1001,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetEndpoint(tt.args.ctx, tt.args.opts, tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetEndpoint() = %v, want %v", got, tt.want)
				t.Error(got.External)
				t.Error(got.Internal)
			}
		})
	}
}
