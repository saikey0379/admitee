# admitee: A kubernetes admission tool for smooth updates
Effective for websocket.
Using the blacklist policy, only deny when results are not matched expected.

## Prepare
### Service transformation
``` shell
# 1.Separate the readiness probe
## The health check logic is the same as the liveness probe, but provides operational capabilities
# 2.Add a control route for readiness probe
## So it can return different httpStatus to isolation traffic
# 3.Adding a route confirms that the replica has processed all the requests
## When the status of the replica is obtained by this interface, Pod can be destroyed smoothly
```

## Getting Started
### image build 
``` shell
# go build -o admiteed cmd/admiteed/main.go
# docker build -t docker.example.com/admiteed:v0.1.0 .
```
### kubernetes set
``` shell
# 1.admitee/deploy/Secret.yaml                         # create pem for svc name
# 2.admitee/deploy/ValidatingWebhookConfiguration.yaml # update caBundle $(base64 -w0 ca.pem)
# 3.admitee/deploy/Deployment.yaml                     # update Deployment start parameter

## apply config
# kubectl apply -f admitee/deploy/
```
### deployment usage example

``` shell
# kubectl apply -f - <<EOF
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
EOF
```
### get smooth
``` shell
# kubectl get smooth
NAME            TARGETREFNAME   TARGETREFKIND   AGE
test            test            Deployment      15h
```
### smoothing logs with pod delete operation
```shell
I0830 14:59:30.744588       1 smooth.go:90] MESSAGE: Smoothing Target [default/Deployment/test]
I0830 14:59:30.744634       1 smooth.go:95] MESSAGE: Smoothing OwnerReference [default/ReplicaSet/test-5b79fff4c7]
I0830 14:59:30.745355       1 smooth.go:116] MESSAGE: Target [default/Deployment/test] Smoothing Count [0]
I0830 14:59:30.757499       1 smooth.go:176] SUCCESS: SET [ADMITEE_SMOOTH_POD_default_test-5b79fff4c7-wr2z5:default_test-5b79fff4c7_60_1661842770_0]
I0830 14:59:30.765155       1 smooth.go:221] MESSAGE: Rule [/isolation] Reason [Post "http://10.244.44.55:8080/isolation": dial tcp 10.244.44.55:80: connect: connection refused]
I0830 14:59:30.772598       1 smooth.go:221] MESSAGE: Rule [/empty] Reason [Get "http://10.244.44.55:8080/empty": dial tcp 10.244.44.55:8080: connect: connection refused]
I0830 14:59:30.773539       1 smooth.go:145] SUCCESS: SET [ADMITEE_SMOOTH_DELETE_default_test-5b79fff4c7-wr2z5:1]
I0830 14:59:30.773564       1 smooth.go:150] MESSAGE: POD [default/test-5b79fff4c7-wr2z5], Delete [true], Reason [{Post "http://10.244.44.55:8080/isolation": dial tcp 10.244.44.55:8080: connect: connection refused},{Get "http://10.244.44.55:8080/empty": dial tcp 10.244.44.55:8080: connect: connection refused}]
...
...
I0830 15:00:04.020087       1 delete_loop.go:135] SUCCESS: DEL[ADMITEE_SMOOTH_POD_default_test-5b79fff4c7-wr2z5]
I0830 15:00:04.020325       1 delete_loop.go:142] SUCCESS: DEL[ADMITEE_SMOOTH_DELETE_default_test-5b79fff4c7-wr2z5]
 
```
### Pod delete 
