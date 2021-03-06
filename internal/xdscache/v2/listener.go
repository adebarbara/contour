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
	"path"
	"sort"
	"sync"

	envoy_api_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoy_api_v2_auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoy_api_v2_listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoy_api_v2_accesslog "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	http "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v2"
	"github.com/golang/protobuf/proto"
	"github.com/projectcontour/contour/internal/contour"
	"github.com/projectcontour/contour/internal/dag"
	envoy_v2 "github.com/projectcontour/contour/internal/envoy/v2"
	"github.com/projectcontour/contour/internal/protobuf"
	"github.com/projectcontour/contour/internal/sorter"
	"github.com/projectcontour/contour/internal/timeout"
	"github.com/projectcontour/contour/pkg/config"
)

// nolint:golint
const (
	ENVOY_HTTP_LISTENER            = "ingress_http"
	ENVOY_FALLBACK_ROUTECONFIG     = "ingress_fallbackcert"
	ENVOY_HTTPS_LISTENER           = "ingress_https"
	DEFAULT_HTTP_ACCESS_LOG        = "/dev/stdout"
	DEFAULT_HTTP_LISTENER_ADDRESS  = "0.0.0.0"
	DEFAULT_HTTP_LISTENER_PORT     = 8080
	DEFAULT_HTTPS_ACCESS_LOG       = "/dev/stdout"
	DEFAULT_HTTPS_LISTENER_ADDRESS = DEFAULT_HTTP_LISTENER_ADDRESS
	DEFAULT_HTTPS_LISTENER_PORT    = 8443
)

// ListenerConfig holds configuration parameters for building Envoy Listeners.
type ListenerConfig struct {
	// Envoy's HTTP (non TLS) listener address.
	// If not set, defaults to DEFAULT_HTTP_LISTENER_ADDRESS.
	HTTPAddress string

	// Envoy's HTTP (non TLS) listener port.
	// If not set, defaults to DEFAULT_HTTP_LISTENER_PORT.
	HTTPPort int

	// Envoy's HTTP (non TLS) access log path.
	// If not set, defaults to DEFAULT_HTTP_ACCESS_LOG.
	HTTPAccessLog string

	// Envoy's HTTPS (TLS) listener address.
	// If not set, defaults to DEFAULT_HTTPS_LISTENER_ADDRESS.
	HTTPSAddress string

	// Envoy's HTTPS (TLS) listener port.
	// If not set, defaults to DEFAULT_HTTPS_LISTENER_PORT.
	HTTPSPort int

	// Envoy's HTTPS (TLS) access log path.
	// If not set, defaults to DEFAULT_HTTPS_ACCESS_LOG.
	HTTPSAccessLog string

	// UseProxyProto configures all listeners to expect a PROXY
	// V1 or V2 preamble.
	// If not set, defaults to false.
	UseProxyProto bool

	// MinimumTLSVersion defines the minimum TLS protocol version the proxy should accept.
	MinimumTLSVersion envoy_api_v2_auth.TlsParameters_TlsProtocol

	// DefaultHTTPVersions defines the default set of HTTP
	// versions the proxy should accept. If not specified, all
	// supported versions are accepted. This is applied to both
	// HTTP and HTTPS listeners but has practical effect only for
	// HTTPS, because we don't support h2c.
	DefaultHTTPVersions []envoy_v2.HTTPVersionType

	// AccessLogType defines if Envoy logs should be output as Envoy's default or JSON.
	// Valid values: 'envoy', 'json'
	// If not set, defaults to 'envoy'
	AccessLogType config.AccessLogType

	// AccessLogFields sets the fields that should be shown in JSON logs.
	// Valid entries are the keys from internal/envoy/accesslog.go:jsonheaders
	// Defaults to a particular set of fields.
	AccessLogFields config.AccessLogFields

	// RequestTimeout configures the request_timeout for all Connection Managers.
	RequestTimeout timeout.Setting

	// ConnectionIdleTimeout configures the common_http_protocol_options.idle_timeout for all
	// Connection Managers.
	ConnectionIdleTimeout timeout.Setting

	// StreamIdleTimeout configures the stream_idle_timeout for all Connection Managers.
	StreamIdleTimeout timeout.Setting

	// MaxConnectionDuration configures the common_http_protocol_options.max_connection_duration for all
	// Connection Managers.
	MaxConnectionDuration timeout.Setting

	// ConnectionShutdownGracePeriod configures the drain_timeout for all Connection Managers.
	ConnectionShutdownGracePeriod timeout.Setting
}

// httpAddress returns the port for the HTTP (non TLS)
// listener or DEFAULT_HTTP_LISTENER_ADDRESS if not configured.
func (lvc *ListenerConfig) httpAddress() string {
	if lvc.HTTPAddress != "" {
		return lvc.HTTPAddress
	}
	return DEFAULT_HTTP_LISTENER_ADDRESS
}

// httpPort returns the port for the HTTP (non TLS)
// listener or DEFAULT_HTTP_LISTENER_PORT if not configured.
func (lvc *ListenerConfig) httpPort() int {
	if lvc.HTTPPort != 0 {
		return lvc.HTTPPort
	}
	return DEFAULT_HTTP_LISTENER_PORT
}

// httpAccessLog returns the access log for the HTTP (non TLS)
// listener or DEFAULT_HTTP_ACCESS_LOG if not configured.
func (lvc *ListenerConfig) httpAccessLog() string {
	if lvc.HTTPAccessLog != "" {
		return lvc.HTTPAccessLog
	}
	return DEFAULT_HTTP_ACCESS_LOG
}

// httpsAddress returns the port for the HTTPS (TLS)
// listener or DEFAULT_HTTPS_LISTENER_ADDRESS if not configured.
func (lvc *ListenerConfig) httpsAddress() string {
	if lvc.HTTPSAddress != "" {
		return lvc.HTTPSAddress
	}
	return DEFAULT_HTTPS_LISTENER_ADDRESS
}

// httpsPort returns the port for the HTTPS (TLS) listener
// or DEFAULT_HTTPS_LISTENER_PORT if not configured.
func (lvc *ListenerConfig) httpsPort() int {
	if lvc.HTTPSPort != 0 {
		return lvc.HTTPSPort
	}
	return DEFAULT_HTTPS_LISTENER_PORT
}

// httpsAccessLog returns the access log for the HTTPS (TLS)
// listener or DEFAULT_HTTPS_ACCESS_LOG if not configured.
func (lvc *ListenerConfig) httpsAccessLog() string {
	if lvc.HTTPSAccessLog != "" {
		return lvc.HTTPSAccessLog
	}
	return DEFAULT_HTTPS_ACCESS_LOG
}

// accesslogType returns the access log type that should be configured
// across all listener types or DEFAULT_ACCESS_LOG_TYPE if not configured.
func (lvc *ListenerConfig) accesslogType() string {
	if lvc.AccessLogType != "" {
		return string(lvc.AccessLogType)
	}
	return string(config.DEFAULT_ACCESS_LOG_TYPE)
}

// accesslogFields returns the access log fields that should be configured
// for Envoy, or a default set if not configured.
func (lvc *ListenerConfig) accesslogFields() config.AccessLogFields {
	if lvc.AccessLogFields != nil {
		return lvc.AccessLogFields
	}
	return config.DefaultFields
}

func (lvc *ListenerConfig) newInsecureAccessLog() []*envoy_api_v2_accesslog.AccessLog {
	switch lvc.accesslogType() {
	case string(config.JSONAccessLog):
		return envoy_v2.FileAccessLogJSON(lvc.httpAccessLog(), lvc.accesslogFields())
	default:
		return envoy_v2.FileAccessLogEnvoy(lvc.httpAccessLog())
	}
}

func (lvc *ListenerConfig) newSecureAccessLog() []*envoy_api_v2_accesslog.AccessLog {
	switch lvc.accesslogType() {
	case "json":
		return envoy_v2.FileAccessLogJSON(lvc.httpsAccessLog(), lvc.accesslogFields())
	default:
		return envoy_v2.FileAccessLogEnvoy(lvc.httpsAccessLog())
	}
}

// minTLSVersion returns the requested minimum TLS protocol
// version or envoy_api_v2_auth.TlsParameters_TLSv1_2 if not configured.
func (lvc *ListenerConfig) minTLSVersion() envoy_api_v2_auth.TlsParameters_TlsProtocol {
	if lvc.MinimumTLSVersion > envoy_api_v2_auth.TlsParameters_TLSv1_2 {
		return lvc.MinimumTLSVersion
	}
	return envoy_api_v2_auth.TlsParameters_TLSv1_2
}

// ListenerCache manages the contents of the gRPC LDS cache.
type ListenerCache struct {
	mu           sync.Mutex
	values       map[string]*envoy_api_v2.Listener
	staticValues map[string]*envoy_api_v2.Listener

	Config ListenerConfig
	contour.Cond
}

// NewListenerCache returns an instance of a ListenerCache
func NewListenerCache(config ListenerConfig, address string, port int) *ListenerCache {
	stats := envoy_v2.StatsListener(address, port)
	return &ListenerCache{
		Config: config,
		staticValues: map[string]*envoy_api_v2.Listener{
			stats.Name: stats,
		},
	}
}

// Update replaces the contents of the cache with the supplied map.
func (c *ListenerCache) Update(v map[string]*envoy_api_v2.Listener) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.values = v
	c.Cond.Notify()
}

// Contents returns a copy of the cache's contents.
func (c *ListenerCache) Contents() []proto.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	var values []*envoy_api_v2.Listener
	for _, v := range c.values {
		values = append(values, v)
	}
	for _, v := range c.staticValues {
		values = append(values, v)
	}
	sort.Stable(sorter.For(values))
	return protobuf.AsMessages(values)
}

// Query returns the proto.Messages in the ListenerCache that match
// a slice of strings
func (c *ListenerCache) Query(names []string) []proto.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	var values []*envoy_api_v2.Listener
	for _, n := range names {
		v, ok := c.values[n]
		if !ok {
			v, ok = c.staticValues[n]
			if !ok {
				// if the listener is not registered in
				// dynamic or static values then skip it
				// as there is no way to return a blank
				// listener because the listener address
				// field is required.
				continue
			}
		}
		values = append(values, v)
	}
	sort.Stable(sorter.For(values))
	return protobuf.AsMessages(values)
}

func (*ListenerCache) TypeURL() string { return resource.ListenerType }

func (c *ListenerCache) OnChange(root *dag.DAG) {
	listeners := visitListeners(root, &c.Config)
	c.Update(listeners)
}

type listenerVisitor struct {
	*ListenerConfig

	listeners map[string]*envoy_api_v2.Listener
	http      bool // at least one dag.VirtualHost encountered
}

func visitListeners(root dag.Vertex, lvc *ListenerConfig) map[string]*envoy_api_v2.Listener {
	lv := listenerVisitor{
		ListenerConfig: lvc,
		listeners: map[string]*envoy_api_v2.Listener{
			ENVOY_HTTPS_LISTENER: envoy_v2.Listener(
				ENVOY_HTTPS_LISTENER,
				lvc.httpsAddress(),
				lvc.httpsPort(),
				secureProxyProtocol(lvc.UseProxyProto),
			),
		},
	}

	lv.visit(root)

	if lv.http {
		// Add a listener if there are vhosts bound to http.
		cm := envoy_v2.HTTPConnectionManagerBuilder().
			Codec(envoy_v2.CodecForVersions(lv.DefaultHTTPVersions...)).
			DefaultFilters().
			RouteConfigName(ENVOY_HTTP_LISTENER).
			MetricsPrefix(ENVOY_HTTP_LISTENER).
			AccessLoggers(lvc.newInsecureAccessLog()).
			RequestTimeout(lvc.RequestTimeout).
			ConnectionIdleTimeout(lvc.ConnectionIdleTimeout).
			StreamIdleTimeout(lvc.StreamIdleTimeout).
			MaxConnectionDuration(lvc.MaxConnectionDuration).
			ConnectionShutdownGracePeriod(lvc.ConnectionShutdownGracePeriod).
			Get()

		lv.listeners[ENVOY_HTTP_LISTENER] = envoy_v2.Listener(
			ENVOY_HTTP_LISTENER,
			lvc.httpAddress(),
			lvc.httpPort(),
			proxyProtocol(lvc.UseProxyProto),
			cm,
		)
	}

	// Remove the https listener if there are no vhosts bound to it.
	if len(lv.listeners[ENVOY_HTTPS_LISTENER].FilterChains) == 0 {
		delete(lv.listeners, ENVOY_HTTPS_LISTENER)
	} else {
		// there's some https listeners, we need to sort the filter chains
		// to ensure that the LDS entries are identical.
		sort.Stable(sorter.For(lv.listeners[ENVOY_HTTPS_LISTENER].FilterChains))
	}

	return lv.listeners
}

func proxyProtocol(useProxy bool) []*envoy_api_v2_listener.ListenerFilter {
	if useProxy {
		return envoy_v2.ListenerFilters(
			envoy_v2.ProxyProtocol(),
		)
	}
	return nil
}

func secureProxyProtocol(useProxy bool) []*envoy_api_v2_listener.ListenerFilter {
	return append(proxyProtocol(useProxy), envoy_v2.TLSInspector())
}

func (v *listenerVisitor) visit(vertex dag.Vertex) {
	max := func(a, b envoy_api_v2_auth.TlsParameters_TlsProtocol) envoy_api_v2_auth.TlsParameters_TlsProtocol {
		if a > b {
			return a
		}
		return b
	}

	switch vh := vertex.(type) {
	case *dag.VirtualHost:
		// we only create on http listener so record the fact
		// that we need to then double back at the end and add
		// the listener properly.
		v.http = true
	case *dag.SecureVirtualHost:
		var alpnProtos []string
		var filters []*envoy_api_v2_listener.Filter

		if vh.TCPProxy == nil {
			var authFilter *http.HttpFilter

			if vh.AuthorizationService != nil {
				authFilter = envoy_v2.FilterExternalAuthz(
					vh.AuthorizationService.Name,
					vh.AuthorizationFailOpen,
					vh.AuthorizationResponseTimeout,
				)
			}

			// Create a uniquely named HTTP connection manager for
			// this vhost, so that the SNI name the client requests
			// only grants access to that host. See RFC 6066 for
			// security advice. Note that we still use the generic
			// metrics prefix to keep compatibility with previous
			// Contour versions since the metrics prefix will be
			// coded into monitoring dashboards.
			filters = envoy_v2.Filters(
				envoy_v2.HTTPConnectionManagerBuilder().
					Codec(envoy_v2.CodecForVersions(v.DefaultHTTPVersions...)).
					AddFilter(envoy_v2.FilterMisdirectedRequests(vh.VirtualHost.Name)).
					DefaultFilters().
					AddFilter(authFilter).
					RouteConfigName(path.Join("https", vh.VirtualHost.Name)).
					MetricsPrefix(ENVOY_HTTPS_LISTENER).
					AccessLoggers(v.ListenerConfig.newSecureAccessLog()).
					RequestTimeout(v.ListenerConfig.RequestTimeout).
					ConnectionIdleTimeout(v.ListenerConfig.ConnectionIdleTimeout).
					StreamIdleTimeout(v.ListenerConfig.StreamIdleTimeout).
					MaxConnectionDuration(v.ListenerConfig.MaxConnectionDuration).
					ConnectionShutdownGracePeriod(v.ListenerConfig.ConnectionShutdownGracePeriod).
					Get(),
			)

			alpnProtos = envoy_v2.ProtoNamesForVersions(v.DefaultHTTPVersions...)
		} else {
			filters = envoy_v2.Filters(
				envoy_v2.TCPProxy(ENVOY_HTTPS_LISTENER,
					vh.TCPProxy,
					v.ListenerConfig.newSecureAccessLog()),
			)

			// Do not offer ALPN for TCP proxying, since
			// the protocols will be provided by the TCP
			// backend in its ServerHello.
		}

		var downstreamTLS *envoy_api_v2_auth.DownstreamTlsContext

		// Secret is provided when TLS is terminated and nil when TLS passthrough is used.
		if vh.Secret != nil {
			// Choose the higher of the configured or requested TLS version.
			vers := max(v.ListenerConfig.minTLSVersion(), vh.MinTLSVersion)

			downstreamTLS = envoy_v2.DownstreamTLSContext(
				vh.Secret,
				vers,
				vh.DownstreamValidation,
				alpnProtos...)
		}

		v.listeners[ENVOY_HTTPS_LISTENER].FilterChains = append(v.listeners[ENVOY_HTTPS_LISTENER].FilterChains,
			envoy_v2.FilterChainTLS(vh.VirtualHost.Name, downstreamTLS, filters))

		// If this VirtualHost has enabled the fallback certificate then set a default
		// FilterChain which will allow routes with this vhost to accept non-SNI TLS requests.
		// Note that we don't add the misdirected requests filter on this chain because at this
		// point we don't actually know the full set of server names that will be bound to the
		// filter chain through the ENVOY_FALLBACK_ROUTECONFIG route configuration.
		if vh.FallbackCertificate != nil && !envoy_v2.ContainsFallbackFilterChain(v.listeners[ENVOY_HTTPS_LISTENER].FilterChains) {
			// Construct the downstreamTLSContext passing the configured fallbackCertificate. The TLS minProtocolVersion will use
			// the value defined in the Contour Configuration file if defined.
			downstreamTLS = envoy_v2.DownstreamTLSContext(
				vh.FallbackCertificate,
				v.ListenerConfig.minTLSVersion(),
				vh.DownstreamValidation,
				alpnProtos...)

			// Default filter chain
			filters = envoy_v2.Filters(
				envoy_v2.HTTPConnectionManagerBuilder().
					DefaultFilters().
					RouteConfigName(ENVOY_FALLBACK_ROUTECONFIG).
					MetricsPrefix(ENVOY_HTTPS_LISTENER).
					AccessLoggers(v.ListenerConfig.newSecureAccessLog()).
					RequestTimeout(v.ListenerConfig.RequestTimeout).
					ConnectionIdleTimeout(v.ListenerConfig.ConnectionIdleTimeout).
					StreamIdleTimeout(v.ListenerConfig.StreamIdleTimeout).
					MaxConnectionDuration(v.ListenerConfig.MaxConnectionDuration).
					ConnectionShutdownGracePeriod(v.ListenerConfig.ConnectionShutdownGracePeriod).
					Get(),
			)

			v.listeners[ENVOY_HTTPS_LISTENER].FilterChains = append(v.listeners[ENVOY_HTTPS_LISTENER].FilterChains,
				envoy_v2.FilterChainTLSFallback(downstreamTLS, filters))
		}

	default:
		// recurse
		vertex.Visit(v.visit)
	}
}
