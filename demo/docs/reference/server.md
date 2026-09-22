# server

## Arguments

The following arguments are supported:

| Name | Type | Format | Description | Required | Default |
|------|------|--------|-------------|----------|---------|
| host | string | - | Address to bind. Use 0.0.0.0 to accept connections from outside the host. | No | localhost |
| port | int | - | TCP port to listen on. Ports below 1024 need elevated privileges. | No | 8080 |
| origins | []string | url | Origins allowed by CORS. Empty allows none. | No | - |

