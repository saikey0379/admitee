# admitee: 一个实现POD平滑更新的Kubernetes工具
可以实现POD平滑退出，对长连接websocket服务有效.
使用黑名单策略，当结果不匹配"expected"的value时拒绝更新

## 准备
### 服务改动
``` shell
# 1.拆分就绪探针
## 需确保健康检查逻辑与存活探针相同，但提供接口控制功能
# 2.为就绪探针开发控制路由
## 通过接口控制就绪探针修改HTTP返回码以隔离流量
# 3.增加一个接口以确保POD隔离流量后，请求均已处理完毕
## 当通过此接口获取副本状态时，POD可以顺利销毁
```

## 开始
### 镜像构建
``` shell
# go build -o admiteed cmd/admiteed/main.go
# docker build -t docker.example.com/admiteed:v0.1.0 .
```
### 部署
``` shell
# 1.admitee/deploy/Secret.yaml                         # 创建service证书
# 2.admitee/deploy/ValidatingWebhookConfiguration.yaml # update caBundle $(base64 -w0 ca.pem)
# 3.admitee/deploy/Deployment.yaml                     # 更新Deployment启动参数

## apply config
# kubectl apply -f admitee/deploy/
```
### deployment 配置示例

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
  interval: 10
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
### 查看配置
``` shell
# kubectl get smooth
NAME            TARGETREFNAME   TARGETREFKIND   AGE
test            test            Deployment      15h
```
### POD滚动更新或删除时，观察服务日志
```shell
I0831 14:13:59.392433       1 server.go:65] Start to listening on http address: 0.0.0.0:443
I0831 14:15:23.103835       1 smooth.go:90] MESSAGE: Smoothing Target [default/Deployment/test] POD [test-756777c86c-qdtm7]
I0831 14:15:23.104444       1 smooth.go:115] MESSAGE: Target [default/Deployment/test] Smoothing Count [0]
I0831 14:15:23.104598       1 smooth.go:90] MESSAGE: Smoothing Target [default/Deployment/test] POD [test-756777c86c-rg4d2]
I0831 14:15:23.137168       1 smooth.go:172] SUCCESS: SET [ADMITEE_SMOOTH_POD_default_test-756777c86c-qdtm7:default_test-756777c86c_10_1661926523_0]
...
I0831 14:15:23.141473       1 smooth.go:137] MESSAGE: POD [default/test-756777c86c-qdtm7], Delete [false], Reason [{post 8080/isolation success},{get 8080/empty false}]
I0831 14:15:24.125969       1 deployment.go:39] MESSAGE: Deployment[test] SmoothCount: 1, MaxUnavailableCount: 2, DesiredNumber：4, MaxUnavailable:50%
I0831 14:15:24.145855       1 smooth.go:172] SUCCESS: SET [ADMITEE_SMOOTH_POD_default_test-756777c86c-rg4d2:default_test-756777c86c_10_1661926524_0]
I0831 14:15:24.150343       1 smooth.go:137] MESSAGE: POD [default/test-756777c86c-rg4d2], Delete [false], Reason [{post 8080/isolation success},{get 8080/empty false}]
I0831 14:15:24.202450       1 smooth.go:137] MESSAGE: POD [default/test-756777c86c-qdtm7], Delete [false], Reason [{post 8080/isolation success},{get 8080/empty false}]
...
I0831 14:16:05.892022       1 smooth.go:137] MESSAGE: POD [default/test-756777c86c-rg4d2], Delete [true], Reason [{post 8080/isolation success},{get 8080/empty success}]
I0831 14:16:05.892640       1 smooth.go:145] SUCCESS: SET [ADMITEE_SMOOTH_DEL_default_test-756777c86c-rg4d2:1]
I0831 14:16:05.917763       1 smooth.go:137] MESSAGE: POD [default/test-756777c86c-rg4d2], Delete [true], Reason [{post 8080/isolation success},{get 8080/empty success}]
I0831 14:16:05.943098       1 smooth.go:137] MESSAGE: POD [default/test-756777c86c-rg4d2], Delete [true], Reason [{post 8080/isolation success},{get 8080/empty success}]
I0831 14:16:06.551268       1 loop_del.go:135] SUCCESS: DEL[ADMITEE_SMOOTH_POD_default_test-756777c86c-rg4d2]
I0831 14:16:06.551510       1 loop_del.go:142] SUCCESS: DEL[ADMITEE_SMOOTH_DEL_default_test-756777c86c-rg4d2]
```
### 
