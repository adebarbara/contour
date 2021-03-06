// Copyright Project Contour Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2

import (
	"testing"

	"github.com/projectcontour/contour/internal/featuretests"

	envoy_api_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	projcontour "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/projectcontour/contour/apis/projectcontour/v1alpha1"
	"github.com/projectcontour/contour/internal/contour"
	"github.com/projectcontour/contour/internal/dag"
	envoyv2 "github.com/projectcontour/contour/internal/envoy/v2"
	"github.com/projectcontour/contour/internal/fixture"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

func proxyClientCertificateOpt(t *testing.T) func(eh *contour.EventHandler) {
	return func(eh *contour.EventHandler) {
		secret := types.NamespacedName{
			Name:      "envoyclientsecret",
			Namespace: "default",
		}

		log := fixture.NewTestLogger(t)
		log.SetLevel(logrus.DebugLevel)

		eh.Builder.Processors = []dag.Processor{
			&dag.IngressProcessor{
				ClientCertificate: &secret,
				FieldLogger:       log.WithField("context", "IngressProcessor"),
			},
			&dag.HTTPProxyProcessor{
				ClientCertificate: &secret,
			},
			&dag.ExtensionServiceProcessor{
				ClientCertificate: &secret,
				FieldLogger:       log.WithField("context", "ExtensionServiceProcessor"),
			},
			&dag.ListenerProcessor{},
		}
	}
}

func clientSecret() *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envoyclientsecret",
			Namespace: "default",
		},
		Type: "kubernetes.io/tls",
		Data: featuretests.Secretdata(featuretests.CERTIFICATE, featuretests.RSA_PRIVATE_KEY),
	}
}

func caSecret() *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backendcacert",
			Namespace: "default",
		},
		Data: map[string][]byte{
			dag.CACertificateKey: []byte(featuretests.CERTIFICATE),
		},
	}
}

func TestBackendClientAuthenticationWithHTTPProxy(t *testing.T) {
	rh, c, done := setup(t, proxyClientCertificateOpt(t))
	defer done()

	sec1 := clientSecret()
	sec2 := caSecret()
	rh.OnAdd(sec1)
	rh.OnAdd(sec2)

	svc := fixture.NewService("backend").
		WithPorts(v1.ServicePort{Name: "http", Port: 443})
	rh.OnAdd(svc)

	proxy := fixture.NewProxy("authenticated").WithSpec(
		projcontour.HTTPProxySpec{
			VirtualHost: &projcontour.VirtualHost{
				Fqdn: "www.example.com",
			},
			Routes: []projcontour.Route{{
				Services: []projcontour.Service{{
					Name:     svc.Name,
					Port:     443,
					Protocol: pointer.StringPtr("tls"),
					UpstreamValidation: &projcontour.UpstreamValidation{
						CACertificate: sec2.Name,
						SubjectName:   "subjname",
					},
				}},
			}},
		})
	rh.OnAdd(proxy)

	c.Request(clusterType).Equals(&v2.DiscoveryResponse{
		Resources: resources(t,
			tlsCluster(cluster("default/backend/443/d411a4160f", "default/backend/http", "default_backend_443"), []byte(featuretests.CERTIFICATE), "subjname", "", sec1),
		),
		TypeUrl: clusterType,
	})

	// Test the error branch when Envoy client certificate secret does not exist.
	rh.OnDelete(sec1)
	c.Request(clusterType).Equals(&v2.DiscoveryResponse{
		Resources: nil,
		TypeUrl:   clusterType,
	})

}

func TestBackendClientAuthenticationWithIngress(t *testing.T) {
	rh, c, done := setup(t, proxyClientCertificateOpt(t))
	defer done()

	sec1 := clientSecret()
	sec2 := caSecret()
	rh.OnAdd(sec1)
	rh.OnAdd(sec2)

	svc := fixture.NewService("backend").
		Annotate("projectcontour.io/upstream-protocol.tls", "443").
		WithPorts(v1.ServicePort{Name: "http", Port: 443})
	rh.OnAdd(svc)

	ingress := &v1beta1.Ingress{
		ObjectMeta: fixture.ObjectMeta("authenticated"),
		Spec: v1beta1.IngressSpec{
			Backend: &v1beta1.IngressBackend{
				ServiceName: "backend",
				ServicePort: intstr.FromInt(443),
			},
		},
	}
	rh.OnAdd(ingress)

	c.Request(clusterType).Equals(&v2.DiscoveryResponse{
		Resources: resources(t,
			tlsClusterWithoutValidation(cluster("default/backend/443/da39a3ee5e", "default/backend/http", "default_backend_443"), "", sec1),
		),
		TypeUrl: clusterType,
	})

	// Test the error branch when Envoy client certificate secret does not exist.
	rh.OnDelete(sec1)
	c.Request(clusterType).Equals(&v2.DiscoveryResponse{
		Resources: nil,
		TypeUrl:   clusterType,
	})
}

func TestBackendClientAuthenticationWithExtensionService(t *testing.T) {
	rh, c, done := setup(t, proxyClientCertificateOpt(t))
	defer done()

	sec1 := clientSecret()
	sec2 := caSecret()
	rh.OnAdd(sec1)
	rh.OnAdd(sec2)

	svc := fixture.NewService("backend").
		WithPorts(v1.ServicePort{Name: "grpc", Port: 6001})
	rh.OnAdd(svc)

	ext := &v1alpha1.ExtensionService{
		ObjectMeta: fixture.ObjectMeta("ext"),
		Spec: v1alpha1.ExtensionServiceSpec{
			Services: []v1alpha1.ExtensionServiceTarget{
				{Name: svc.Name, Port: 6001},
			},
			UpstreamValidation: &projcontour.UpstreamValidation{
				CACertificate: sec2.Name,
				SubjectName:   "subjname",
			},
		},
	}

	rh.OnAdd(ext)

	tlsSocket := envoyv2.UpstreamTLSTransportSocket(
		envoyv2.UpstreamTLSContext(
			&dag.PeerValidationContext{
				CACertificate: &dag.Secret{Object: &v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "default",
					},
					Type: "kubernetes.io/tls",
					Data: map[string][]byte{dag.CACertificateKey: []byte(featuretests.CERTIFICATE)},
				}},
				SubjectName: "subjname"},
			"subjname",
			&dag.Secret{Object: sec1},
			"h2",
		),
	)
	c.Request(clusterType).Equals(&envoy_api_v2.DiscoveryResponse{
		TypeUrl: clusterType,
		Resources: resources(t,
			DefaultCluster(
				h2cCluster(cluster("extension/default/ext", "extension/default/ext", "extension_default_ext")),
				&envoy_api_v2.Cluster{TransportSocket: tlsSocket},
			),
		),
	})

	// Test the error branch when Envoy client certificate secret does not exist.
	rh.OnDelete(sec1)
	c.Request(clusterType).Equals(&v2.DiscoveryResponse{
		Resources: nil,
		TypeUrl:   clusterType,
	})
}
