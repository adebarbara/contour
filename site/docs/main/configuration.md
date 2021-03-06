# Contour Configuration Reference

- [Configuration File](#configuration-file)
- [Environment Variables](#environment-variables)
- [Bootstrap Config File](#bootstrap-config-file)

## Configuration File

A configuration file can be passed to the `--config-path` argument of the `contour serve` command to specify additional configuration to Contour.
In most deployments, this file is passed to Contour via a ConfigMap which is mounted as a volume to the Contour pod.

The Contour configuration file is optional.
In its absence, Contour will operate with reasonable defaults.
Where Contour settings can also be specified with command-line flags, the command-line value takes precedence over the configuration file.

| Field Name | Type | Default | Description |
|------------|------|---------|-------------|
| accesslog-format | string | `envoy` | This key sets the global [access log format][2] for Envoy. Valid options are `envoy` or `json`. |
| debug | boolean | `false` | Enables debug logging. |
| default-http-versions | string array | <code style="white-space:nowrap">HTTP/1.1</code> <br> <code style="white-space:nowrap">HTTP/2</code> | This array specifies the HTTP versions that Contour should program Envoy to serve. HTTP versions are specified as strings of the form "HTTP/x", where "x" represents the version number. |
| disablePermitInsecure | boolean | `false` | If this field is true, Contour will ignore `PermitInsecure` field in HTTPProxy documents. |
| envoy-service-name | string | `envoy` | This sets the service name that will be inspected for address details to be applied to Ingress objects. |
| envoy-service-namespace | string | `projectcontour` | This sets the namespace of the service that will be inspected for address details to be applied to Ingress objects. If the `CONTOUR_NAMESPACE` environment variable is present, Contour will populate this field with its value. |
| ingress-status-address | string | None | If present, this specifies the address that will be copied into the Ingress status for each Ingress that Contour manages. It is exclusive with `envoy-service-name` and `envoy-service-namespace`.|
| incluster | boolean | `false` | This field specifies that Contour is running in a Kubernetes cluster and should use the in-cluster client access configuration.  |
| json-fields | string array | [fields][5]| This is the list the field names to include in the JSON [access log format][2]. |
| kubeconfig | string | `$HOME/.kube/config` | Path to a Kubernetes [kubeconfig file][3] for when Contour is executed outside a cluster. |
| leaderelection | leaderelection | | The [leader election configuration](#leader-election-configuration). |
| tls | TLS | | The default [TLS configuration](#tls-configuration). |
| timeouts | TimeoutConfig | | The [timeout configuration](#timeout-configuration). |
| cluster | ClusterConfig | | The [cluster configuration](#cluster-configuration). |
| server | ServerConfig |  | The [server configuration](#server-configuration) for `contour serve` command. |
{: class="table thead-dark table-bordered"}
<br>

### TLS Configuration

The TLS configuration block can be used to configure default values for how
Contour should provision TLS hosts.

| Field Name | Type| Default  | Description |
|------------|-----|----------|-------------|
| minimum-protocol-version| string | `1.2` | This field specifies the minimum TLS protocol version that is allowed. Valid options are `1.1`, `1.2` (default) and `1.3`. Any other value defaults to TLS 1.2. |
| fallback-certificate | | | [Fallback certificate configuration](#fallback-certificate). |
| envoy-client-certificate | | | [Client certificate configuration for Envoy](#envoy-client-certificate). |
{: class="table thead-dark table-bordered"}
<br>

### Fallback Certificate

| Field Name | Type| Default  | Description |
|------------|-----|----------|-------------|
| name       | string | `""` | This field specifies the name of the Kubernetes secret to use as the fallback certificate.      |
| namespace  | string | `""` | This field specifies the namespace of the Kubernetes secret to use as the fallback certificate. |
{: class="table thead-dark table-bordered"}
<br>

### Envoy Client Certificate

| Field Name | Type| Default  | Description |
|------------|-----|----------|-------------|
| name       | string | `""` | This field specifies the name of the Kubernetes secret to use as the client certificate and private key when establishing TLS connections to the backend service. |
| namespace  | string | `""` | This field specifies the namespace of the Kubernetes secret to use as the client certificate and private key when establishing TLS connections to the backend service. |
{: class="table thead-dark table-bordered"}
<br>

### Leader Election Configuration

The leader election configuration block configures how a deployment with more than one Contour pod elects a leader.
The Contour leader is responsible for updating the status field on Ingress and HTTPProxy documents.
In the vast majority of deployments, only the `configmap-name` and `configmap-namespace` fields should require any configuration.

| Field Name | Type | Default | Description |
|------------|------|---------|-------------|
| configmap-name | string | `leader-elect` | The name of the ConfigMap that Contour leader election will lease. |
| configmap-namespace | string | `projectcontour` | The namespace of the ConfigMap that Contour leader election will lease. If the `CONTOUR_NAMESPACE` environment variable is present, Contour will populate this field with its value. |
| lease-duration | [duration][4] | `15s` | The duration of the leadership lease. |
| renew-deadline | [duration][4] | `10s` | The length of time that the leader will retry refreshing leadership before giving up. |
| retry-period | [duration][4] | `2s` | The interval at which Contour will attempt to the acquire leadership lease. |
{: class="table thead-dark table-bordered"}
<br>

### Timeout Configuration

The timeout configuration block can be used to configure various timeouts for the proxies. All fields are optional; Contour/Envoy defaults apply if a field is not specified.

| Field Name | Type| Default  | Description |
|------------|-----|----------|-------------|
| request-timeout | string | none* | This field specifies the default request timeout. Note that this is a timeout for the entire request, not an idle timeout. Must be a [valid Go duration string][4], or omitted or set to `infinity` to disable the timeout entirely. See [the Envoy documentation][12] for more information.<br /><br />_Note: A value of `0s` previously disabled this timeout entirely. This is no longer the case. Use `infinity` or omit this field to disable the timeout._  |
| connection-idle-timeout| string | `60s` | This field defines how long the proxy should wait while there are no active requests (for HTTP/1.1) or streams (for HTTP/2) before terminating an HTTP connection. Must be a [valid Go duration string][4], or `infinity` to disable the timeout entirely. See [the Envoy documentation][8] for more information. |
| stream-idle-timeout| string | `5m`* |This field defines how long the proxy should wait while there is no request activity (for HTTP/1.1) or stream activity (for HTTP/2) before terminating the HTTP request or stream. Must be a [valid Go duration string][4], or `infinity` to disable the timeout entirely. See [the Envoy documentation][9] for more information. |
| max-connection-duration | string | none* | This field defines the maximum period of time after an HTTP connection has been established from the client to the proxy before it is closed by the proxy, regardless of whether there has been activity or not. Must be a [valid Go duration string][4], or omitted or set to `infinity` for no max duration. See [the Envoy documentation][10] for more information. |
| connection-shutdown-grace-period | string | `5s`* | This field defines how long the proxy will wait between sending an initial GOAWAY frame and a second, final GOAWAY frame when terminating an HTTP/2 connection. During this grace period, the proxy will continue to respond to new streams. After the final GOAWAY frame has been sent, the proxy will refuse new streams. Must be a [valid Go duration string][4]. See [the Envoy documentation][11] for more information. |
{: class="table thead-dark table-bordered"}
<br>
_* This is Envoy's default setting value and is not explicitly configured by Contour._

### Cluster Configuration

The cluster configuration block can be used to configure various parameters for Envoy clusters.

| Field Name | Type| Default  | Description |
|------------|-----|----------|-------------|
| dns-lookup-family | string | auto | This field specifies the dns-lookup-family to use for upstream requests to externalName type Kubernetes services from an HTTPProxy route. Values are: `auto`, `v4, `v6` |
{: class="table thead-dark table-bordered"}
<br>

### Server Configuration

The server configuration block can be used to configure various settings for the `contour serve` command.

| Field Name | Type| Default  | Description |
|------------|-----|----------|-------------|
| xds-server-type | string | contour | This field specifies the xDS Server to use. Options are `contour` or `envoy`.  |
{: class="table thead-dark table-bordered"}
<br>

### Configuration Example

The following is an example ConfigMap with configuration file included:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: contour
  namespace: projectcontour
data:
  contour.yaml: |
    # server:
    #   determine which XDS Server implementation to utilize in Contour.
    #   xds-server-type: contour
    #
    # should contour expect to be running inside a k8s cluster
    # incluster: true
    #
    # path to kubeconfig (if not running inside a k8s cluster)
    # kubeconfig: /path/to/.kube/config
    #
    # disable httpproxy permitInsecure field
    # disablePermitInsecure: false
    tls:
      # minimum TLS version that Contour will negotiate
      # minimumProtocolVersion: "1.1"
      fallback-certificate:
      # name: fallback-secret-name
      # namespace: projectcontour
      envoy-client-certificate:
      # name: envoy-client-cert-secret-name
      # namespace: projectcontour
    # The following config shows the defaults for the leader election.
    # leaderelection:
      # configmap-name: leader-elect
      # configmap-namespace: projectcontour
    # Default HTTP versions.
    # default-http-versions:
    # - "HTTP/1.1"
    # - "HTTP/2"
    # The following shows the default proxy timeout settings.
    # timeouts:
    #  request-timeout: infinity
    #  connection-idle-timeout: 60s
    #  stream-idle-timeout: 5m
    #  max-connection-duration: infinity
    #  connection-shutdown-grace-period: 5s
    #
    # Envoy cluster settings.
    # cluster:
    #   configure the cluster dns lookup family
    #   valid options are: auto (default), v4, v6
    #   dns-lookup-family: auto
```

_Note:_ The default example `contour` includes this [file][1] for easy deployment of Contour.

## Environment Variables

### CONTOUR_NAMESPACE

If present, the value of the `CONTOUR_NAMESPACE` environment variable is used as:

1. The value for the `contour bootstrap --namespace` flag unless otherwise specified.
1. The value for the `contour certgen --namespace` flag unless otherwise specified.
1. The value for the `contour serve --envoy-service-namespace` flag unless otherwise specified.
1. The value for the `leaderelection.configmap-namespace` config file setting for `contour serve` unless otherwise specified.

The `CONTOUR_NAMESPACE` environment variable is set via the [Downward API][6] in the Contour [example manifests][7].

## Bootstrap Config File

The bootstrap configuration file is generated by an initContainer in the Envoy daemonset which runs the `contour bootstrap` command to generate the file.
This configuration file configures the Envoy container to connect to Contour and receive configuration via xDS.

The next section outlines all the available flags that can be passed to the `contour bootstrap` command which are used to customize
the configuration file to match the environment in which Envoy is deployed. 

### Flags

There are flags that can be passed to `contour boostrap` that help configure how Envoy
connects to Contour:

| Flag | Default  | Description |
|------------|----------|-------------|
| <nobr>--resources-dir</nobr> | "" | Directory where resource files will be written.  |
| <nobr>--admin-address</nobr> | 127.0.0.1 | Address the Envoy admin webpage will listen on.  |
| <nobr>--admin-port</nobr> | 9001 | Port the Envoy admin webpage will listen on.  |
| <nobr>--xds-address</nobr> | 127.0.0.1 | Address to connect to Contour xDS server on.  |
| <nobr>--xds-port</nobr> | 8001 | Port to connect to Contour xDS server on. |
| <nobr>--envoy-cafile</nobr> | "" | CA filename for Envoy secure xDS gRPC communication.  |
| <nobr>--envoy-cert-file</nobr> | "" | Client certificate filename for Envoy secure xDS gRPC communication.  |
| <nobr>--envoy-key-file</nobr> | "" | Client key filename for Envoy secure xDS gRPC communication.  |
| <nobr>--namespace</nobr> | projectcontour | Namespace the Envoy container will run, also configured via ENV variable "CONTOUR_NAMESPACE". Namespace is used as part of the metric names on static resources defined in the bootstrap configuration file.    |
| <nobr>--xds-resource-version</nobr> | v3 | Currently, the only valid xDS API resource version is `v3`.  |
{: class="table thead-dark table-bordered"}
<br>


[1]: {{site.github.repository_url}}/tree/{{page.version}}/examples/contour/01-contour-config.yaml
[2]: /guides/structured-logs
[3]: https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/
[4]: https://golang.org/pkg/time/#ParseDuration
[5]: https://godoc.org/github.com/projectcontour/contour/internal/envoy#DefaultFields
[6]: https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/
[7]: {{site.github.repository_url}}/tree/{{page.version}}/examples/contour
[8]: https://www.envoyproxy.io/docs/envoy/latest/api-v2/api/v2/core/protocol.proto#envoy-api-field-core-httpprotocoloptions-idle-timeout
[9]: https://www.envoyproxy.io/docs/envoy/latest/api-v2/config/filter/network/http_connection_manager/v2/http_connection_manager.proto#envoy-api-field-config-filter-network-http-connection-manager-v2-httpconnectionmanager-stream-idle-timeout
[10]: https://www.envoyproxy.io/docs/envoy/latest/api-v2/api/v2/core/protocol.proto#envoy-api-field-core-httpprotocoloptions-max-connection-duration
[11]: https://www.envoyproxy.io/docs/envoy/latest/api-v2/config/filter/network/http_connection_manager/v2/http_connection_manager.proto#envoy-api-field-config-filter-network-http-connection-manager-v2-httpconnectionmanager-drain-timeout
[12]: https://www.envoyproxy.io/docs/envoy/latest/api-v2/config/filter/network/http_connection_manager/v2/http_connection_manager.proto#envoy-api-field-config-filter-network-http-connection-manager-v2-httpconnectionmanager-request-timeout
