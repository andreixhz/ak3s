# AK3S - Kubernetes Cluster Manager

AK3S is a CLI tool for managing Kubernetes clusters across different providers. Currently supports local Docker provider, with plans to support AWS, IBM Cloud, and others.

## Installation

```bash
go install github.com/andreixhz/ak3s@latest
```

## Usage

### Cluster Management

Create a new cluster:
```bash
ak3s cluster create my-cluster
```

List all clusters:
```bash
ak3s cluster list
```

Delete a cluster:
```bash
ak3s cluster delete my-cluster
```

### Node Management

Add a node to a cluster:
```bash
ak3s node add my-cluster my-node
```

Remove a node from a cluster:
```bash
ak3s node remove my-cluster my-node
```

## Requirements

- Docker installed and running
- Go 1.21 or later

## Development

To build the project:
```bash
go build
```

To run tests:
```bash
go test ./...
```
