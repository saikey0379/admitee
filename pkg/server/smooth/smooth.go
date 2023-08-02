package smooth

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"admitee/pkg/api/v1alpha1"
	"admitee/pkg/model"
	"admitee/pkg/utils"

	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/apis/core/v1"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter = runtime.ObjectDefaulter(runtimeScheme)
	podconf   = make(map[string]bool)
)

type SmoothManager struct {
	Config        v1alpha1.Smooth
	ClientRedis   *model.AdmiteeRedisClient
	ClientSmooth  dynamic.Interface
	ClientKubeSet *kubernetes.Clientset
	Ctx           context.Context
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)
	// defaulting with webhooks:
	// https://github.com/kubernetes/kubernetes/issues/57982
	_ = v1.AddToScheme(runtimeScheme)
}

// ValidatingAdmissionWebhook
func (sm *SmoothManager) EnterSmoothProcess(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request
	var allowed bool

	//非POD请求，拒绝
	if req.Kind.Kind != "Pod" {
		return returnAdmissionResponse(allowed, "FAILURE: KIND["+req.Kind.Kind+"]")
	}
	//非删除操作，拒绝
	if req.Operation != "DELETE" {
		return returnAdmissionResponse(allowed, "FAILURE: OPERATION["+string(req.Operation)+"]")
	}

	var pod corev1.Pod
	err := json.Unmarshal(req.OldObject.Raw, &pod)
	if err != nil {
		glog.Errorf("FAILURE: POD[%v], Unmarshal[false], Resource[%v]", req.Namespace+"/"+req.Name, req)
		return returnAdmissionResponse(allowed, "FAILURE: POD Unmarshal["+err.Error()+"]")
	}

	if pod.ObjectMeta.DeletionTimestamp != nil {
		return returnAdmissionResponse(true, "{pod DeletionTimestamp not null}")
	}

	if pod.Status.Phase != "Running" {
		switch pod.Status.Phase {
		case "Pending":
			return returnAdmissionResponse(true, "{pod status "+string(pod.Status.Phase)+"}")
		case "Failed":
			return returnAdmissionResponse(true, "{pod status "+string(pod.Status.Phase)+"/"+string(pod.Status.Reason)+"}")
		}
	}

	var namespace = pod.Namespace
	var namePod = pod.Name
	var reason string

	var keyPOD = "ADMITEE_SMOOTH_POD_" + namespace + "_" + namePod
	valuePOD, _ := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, keyPOD).Result()

	var keySmLabeled = "ADMITEE_SMOOTH_LABEL_" + namespace + "_" + namePod
	valueSmLabeled, _ := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, keySmLabeled).Result()

	if valuePOD != "" || valueSmLabeled != "" {
		allowed, reason = sm.SmoothConfigExec(pod)
	} else {
		// POD首次删除
		_, _, err := sm.GetTarget(pod)
		if err != nil {
			glog.Errorf("FAILURE: Get Target[%s/%s], %v", namespace, namePod, err)
			return returnAdmissionResponse(allowed, err.Error())
		}
		// Lock this request
		kindOwnerReference, nameOwnerReference, _ := utils.GetOwnerReference(pod)
		key := "LOCK_" + kindOwnerReference + "_" + namespace + "_" + nameOwnerReference
		for {
			boolLock := sm.ClientRedis.Lock(key)
			if !boolLock {
				time.Sleep(time.Duration(1) * time.Second)
				continue
			} else {
				break
			}
		}
		glog.Infof("MESSAGE: Smoothing Target[%s] POD[%s]", namespace+"/"+kindOwnerReference+"/"+nameOwnerReference, namePod)

		var boolPodDelete bool
		// count smoothing pods
		countUpdate, err := sm.CountSmoothingPodsByOwnerReferenceName(namespace, nameOwnerReference)
		if err != nil {
			return returnAdmissionResponse(allowed, err.Error())
		}

		if countUpdate < 1 || pod.Labels[v1alpha1.LabelForce] == "true" || pod.Labels[v1alpha1.LabelForce] == "1" {
			boolPodDelete = true
			glog.Infof("MESSAGE: Target[%s] Smoothing Count[0]", namespace+"/"+kindOwnerReference+"/"+nameOwnerReference)
		} else {
			//确定副本是否允许删除
			switch kindOwnerReference {
			case "DaemonSet":
				boolPodDelete, reason = sm.VerifyDeletePodDaemonSet(namespace, nameOwnerReference, countUpdate)
			case "ReplicaSet":
				boolPodDelete, reason = sm.VerifyDeletePodReplicaSet(namespace, nameOwnerReference, countUpdate)
			}
		}

		if boolPodDelete {
			// 已存在POD记录，执行平滑过程
			allowed, reason = sm.SmoothConfigExec(pod)
		}
		// Release the lock
		unLock := sm.ClientRedis.UnLock(key)
		if unLock != 1 {
			glog.Errorf("FAILURE: UnLock[" + key + "]")
		}
	}

	glog.Infof("MESSAGE: POD[%v],Delete[%v],Reason[%v]", req.Namespace+"/"+req.Name, allowed, reason)
	if allowed {
		key := "ADMITEE_SMOOTH_DEL_" + req.Namespace + "_" + req.Name
		vauleDelete, _ := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, key).Result()
		if vauleDelete == "" {
			value := "1"
			err = sm.ClientRedis.Client.SetNX(sm.ClientRedis.Ctx, key, value, 0).Err()
			if err == nil {
				glog.Infof("SUCCESS: SET[%s:%s]", key, value)
			}
		}
	}
	return returnAdmissionResponse(allowed, reason)
}

func (sm *SmoothManager) SmoothConfigExec(pod corev1.Pod) (bool, string) {
	var keySmLabeled = "ADMITEE_SMOOTH_LABEL_" + pod.Namespace + "_" + pod.Name
	valueSmLabeled, _ := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, keySmLabeled).Result()

	var smConfig *v1alpha1.Smooth
	if valueSmLabeled != "" {
		if err := json.Unmarshal([]byte(valueSmLabeled), &smConfig); err != nil {
			return false, err.Error()
		}
	} else {
		var err error
		smConfig, err = sm.GetSmoothConfig(pod)
		if err != nil {
			return false, err.Error()
		}
	}

	if smConfig == nil {
		return true, fmt.Sprintf("Smooth Config NOT SET[%s/%s]", pod.Namespace, pod.Name)
	}

	var interval, timeout int
	_, err := sm.ClientKubeSet.CoreV1().Pods(pod.Namespace).Get(sm.Ctx, pod.Name, metav1.GetOptions{})
	if err == nil {
		if smConfig != nil && smConfig.Spec.Interval > 0 {
			interval = smConfig.Spec.Interval
		}
		if smConfig != nil && smConfig.Spec.Timeout > 0 {
			timeout = smConfig.Spec.Timeout
		}
	}

	if interval == 0 {
		interval = v1alpha1.DefaultInterval
	}
	if timeout == 0 {
		timeout = v1alpha1.DefaultTimeout
	}

	var keyPod = "ADMITEE_SMOOTH_POD_" + pod.Namespace + "_" + pod.Name
	vaulePOD, _ := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, keyPod).Result()
	if vaulePOD == "" && len(pod.GetOwnerReferences()) == 1 {
		value := pod.Namespace + "_" + pod.GetOwnerReferences()[0].Name + "_" + strconv.Itoa(interval) + "_" + strconv.Itoa(timeout) + "_" + strconv.FormatInt(time.Now().Unix(), 10) + "_0"
		err := sm.ClientRedis.Client.SetNX(sm.ClientRedis.Ctx, keyPod, value, 0).Err()
		if err == nil {
			glog.Infof("SUCCESS: SET[%s:%s]", keyPod, value)
		}
	}

	var allowed = true
	var reasons []string
	for _, rule := range smConfig.Spec.Rules {
		if rule.Port >= 65535 {
			glog.Errorf("FAILURE: Port OutOfRange 0~65535 [%v]", rule.Port)
			return false, fmt.Sprintf("FAILURE: Port OutOfRange 0~65535 [%v]", rule.Port)
		} else if rule.Port == 0 {
			rule.Port = int(pod.Spec.Containers[0].Ports[0].ContainerPort)
			if rule.Port == 0 {
				rule.Port = v1alpha1.DefaultPort
			}
		}

		if rule.Path == "" {
			return false, fmt.Sprintf("FAILURE: Path NOT SET[%v]", rule)
		}

		var url string
		if rule.Address != "" {
			url = "http://" + rule.Address + ":" + strconv.Itoa(rule.Port) + rule.Path
		} else {
			url = "http://" + pod.Status.PodIP + ":" + strconv.Itoa(rule.Port) + rule.Path
		}

		if rule.Method == "" {
			rule.Method = v1alpha1.DefaultMethod
		}

		var respStr string
		var err error
		switch rule.Method {
		case "get", "Get", "GET":
			respStr, err = utils.RestApiGet(url)
		case "post", "Post", "POST":
			if rule.Body == "" {
				glog.Errorf("FAILURE: Body NOT SET[%v]", rule)
				return false, fmt.Sprintf("FAILURE: Body NOT SET[%v]", rule)
			}
			respStr, err = utils.RestApiPost(url, rule.Body)
		}

		if err != nil {
			reasons = append(reasons, "{"+err.Error()+"}")
		} else {
			reasons = append(reasons, "{"+rule.Method+" "+strconv.Itoa(rule.Port)+rule.Path+" "+respStr+"}")
			if respStr != strings.TrimSpace(rule.Expect) {
				allowed = false
			}
		}

		if !allowed {
			break
		}
	}

	//Rod状态
	var healthz bool
	for _, i := range pod.Status.Conditions {
		if i.Type == "Ready" && i.Status == "True" {
			reasons = append(reasons, "{pod status "+string(i.Type)+"}")
			healthz = true
			break
		}
	}

	//流量已隔离，修改pod标签，避免影响副本计数
	if !healthz && smConfig.Spec.SmLabel != "" {
		if pod.Labels[smConfig.Spec.SmLabel] != "smoothed" {
			pod.Labels[smConfig.Spec.SmLabel] = "smoothed"
			playLoadBytes, _ := json.Marshal(map[string]interface{}{"metadata": map[string]map[string]string{"labels": pod.Labels}})
			_, err := sm.ClientKubeSet.CoreV1().Pods(pod.Namespace).Patch(sm.Ctx, pod.Name, types.StrategicMergePatchType, playLoadBytes, metav1.PatchOptions{})
			if err != nil {
				reasons = append(reasons, "{smoothLabel set ["+err.Error()+"]}")
				allowed = false
			} else {
				if valueSmLabeled == "" {
					smConfigByte, err := json.Marshal(smConfig)
					if err != nil {
						glog.Infof("FAILURE: Marshal SmConfig[%s:%s]", smConfig, err.Error())
						reasons = append(reasons, "{SmConfig Marshal ["+err.Error()+"]}")
						allowed = false
					}
					err = sm.ClientRedis.Client.SetNX(sm.ClientRedis.Ctx, keySmLabeled, string(smConfigByte), 0).Err()
					if err != nil {
						glog.Infof("FAILURE: SET[%s:%s]", keySmLabeled, valueSmLabeled)
						reasons = append(reasons, "{SmConfig set ["+err.Error()+"]}")
						allowed = false
					}
				}
			}
		}
	}

	if !allowed || healthz {
		return false, strings.Join(reasons, ",")
	} else {
		//避免Terminal状态网络回收对请求的影响
		var keyPodNotReady = "ADMITEE_SMOOTH_NOTREADY_" + pod.Namespace + "_" + pod.Name
		vaulePodNotReady, _ := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, keyPodNotReady).Result()
		if vaulePodNotReady == "" {
			time.Sleep(time.Duration(5) * time.Second)

			value := strconv.FormatInt(time.Now().Unix(), 10)
			err := sm.ClientRedis.Client.SetNX(sm.ClientRedis.Ctx, keyPodNotReady, value, 0).Err()
			if err == nil {
				glog.Infof("SUCCESS: SET[%s:%s]", keyPodNotReady, value)
			}
		}
	}

	return allowed, strings.Join(reasons, ",")
}

func (sm *SmoothManager) GetSmoothConfig(pod corev1.Pod) (*v1alpha1.Smooth, error) {
	namespace := pod.Namespace
	kindTarget, nameTarget, err := sm.GetTarget(pod)
	if err != nil {
		return nil, err
	}

	var gvr = schema.GroupVersionResource{
		Group:    v1alpha1.Group,
		Version:  v1alpha1.Version,
		Resource: v1alpha1.Resource,
	}
	list, err := sm.ClientSmooth.Resource(gvr).Namespace(namespace).List(sm.Ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	data, err := list.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var smList v1alpha1.SmoothList
	if err := json.Unmarshal(data, &smList); err != nil {
		return nil, err
	}

	for _, sm := range smList.Items {
		if sm.Spec.TargetRef.Name == nameTarget && sm.Spec.TargetRef.Kind == kindTarget {
			return &sm, err
		}
	}

	return nil, err
}

func (sm *SmoothManager) GetTarget(pod corev1.Pod) (targetKind string, targetName string, err error) {
	orKind, orName, err := utils.GetOwnerReference(pod)
	if err != nil {
		return "", "", err
	}

	switch orKind {
	case "DaemonSet":
		return orKind, orName, err
	case "ReplicaSet":
		rsName := orName
		rs, err := sm.ClientKubeSet.AppsV1().ReplicaSets(pod.Namespace).Get(sm.Ctx, rsName, metav1.GetOptions{})
		if len(rs.GetOwnerReferences()) == 1 {
			return rs.OwnerReferences[0].Kind, rs.OwnerReferences[0].Name, err
		} else if len(rs.GetOwnerReferences()) < 1 {
			return orKind, orName, err
		} else {
			return "", "", fmt.Errorf("FAILURE: Too Many Target Matched")
		}
	}
	return "", "", err
}

func (sm *SmoothManager) CountSmoothingPodsByOwnerReferenceName(namespace string, ownerReferenceName string) (int, error) {
	var countUpdate int
	var err error

	// get keys of smoothing pods by ownerReferenceName reg
	var keys []string
	reg := "ADMITEE_SMOOTH_POD_" + namespace + "_" + ownerReferenceName + "*"
	keys, err = sm.ClientRedis.Client.Keys(sm.ClientRedis.Ctx, reg).Result()
	if err != nil {
		glog.Errorf("FAILURE: POD KEYS[%s]: %v", reg, err)
	}

	// match pod by ownerReferenceName
	for _, v := range keys {
		var result string
		result, err = sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, v).Result()
		if err != nil {
			glog.Errorf("FAILURE: GET[%s]: %v", v, err)
			return countUpdate, err
		}
		podInfo := strings.Split(result, "_")
		if podInfo[1] == ownerReferenceName {
			countUpdate++
		}
	}
	return countUpdate, err
}

func returnAdmissionResponse(allowed bool, reason string) *v1beta1.AdmissionResponse {
	var result *metav1.Status
	result = &metav1.Status{
		Reason: metav1.StatusReason(reason),
	}

	return &v1beta1.AdmissionResponse{
		Allowed: allowed,
		Result:  result,
	}
}
