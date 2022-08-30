# admitee: A kubernetes admission tool for smooth updates
Effective for websocket

## Getting Started
### image build 
``` shell
# go build -o admiteed cmd/admiteed/main.go
# docker build -t docker.example.com/admiteed:v0.1.0 .
```
### kubernetes set
``` shell
# kubectl apply -f deploy/
```
### deployment usage example
``` yaml
apiVersion: validating.example.com/v1alpha1
kind: Smooth
metadata:
  name: test
  namespace: default
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test
  rules:
    - address: "manage.example.com"
      path: "/UpdateIsLock"
      method: "post"
      body: "deploymentName"
      expect: "false"
    - port: "8080"
      path: "/isolation"
      method: "post"
      body: "true"
      expect: "success"
    - port: "8080"
      path: "/empty"
      method: "get"
      expect: "success"
```