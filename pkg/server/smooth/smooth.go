package smooth

import (
	"fmt"
	"context"
	"encoding/json"
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
	Config            v1alpha1.Smooth
	ClientKubeDynamic dynamic.Interface
	ClientKubeSet     *kubernetes.Clientset
	ClientRedis       *model.AdmiteeRedisClient
	Ctx               context.Context
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
		return returnAdmissionResponse(allowed, "FAILURE: KIND ["+req.Kind.Kind+"]")
	}
	//非删除操作，拒绝
	if req.Operation != "DELETE" {
		return returnAdmissionResponse(allowed, "FAILURE: OPERATION ["+string(req.Operation)+"]")
	}

	var pod corev1.Pod
	err := json.Unmarshal(req.OldObject.Raw, &pod)
	if err != nil {
		glog.Errorf("FAILURE: POD [%v], Unmarshal [false], Resource [%v]", req.Namespace+"/"+req.Name, req)
		return returnAdmissionResponse(allowed, "FAILURE: POD Unmarshal ["+err.Error()+"]")
	}

	var namespace = pod.Namespace
	var namePod   = pod.Name
	var reason string

	keyPOD := "ADMITEE_SMOOTH_POD_" + namespace + "_" + namePod
	valuePOD, _ := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, keyPOD).Result()

	if valuePOD != "" {
		allowed, reason = sm.SmoothConfigExec(pod)
	} else {
		// POD首次删除
		kindTarget, nameTarget, err := sm.GetTarget(pod)
		if err != nil {
			glog.Errorf("FAILURE: Get Target [%s/%s], %v", namespace, namePod, err)
			return returnAdmissionResponse(allowed, err.Error())
		}
		glog.Infof("MESSAGE: Smoothing Target [%s]", namespace + "/" + kindTarget + "/" + nameTarget)
		// Lock this request
		or := pod.GetOwnerReferences()
		nameOwnerReference := or[0].Name
		kindOwnerReference := or[0].Kind
		glog.Infof("MESSAGE: Smoothing OwnerReference [%s]", namespace + "/" + kindOwnerReference + "/" + nameOwnerReference)
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

		var boolPodDelete bool
		// count smoothing pods
		countUpdate, err := sm.CountSmoothingPodsByOwnerReferenceName(namespace, nameOwnerReference)
		if err != nil {
			return returnAdmissionResponse(allowed, err.Error())
		} 

		if countUpdate < 1{
			boolPodDelete = true
			glog.Infof("MESSAGE: Target [%s] Smoothing Count [0]", namespace + "/" + kindTarget + "/" + nameTarget)
		} else {
			//确定副本是否允许删除
			switch kindTarget {
			case "DaemonSet":
				boolPodDelete, reason = sm.VerifyDeletePodDaemonSet(namespace, nameTarget, countUpdate)
			case "Deployment":
				boolPodDelete, reason = sm.VerifyDeletePodDeployment(namespace, nameTarget, countUpdate)
			}
		}

		if boolPodDelete {
			// 已存在POD记录，执行平滑过程
			allowed, reason = sm.SmoothConfigExec(pod)
		}
		// Release the lock
		unLock := sm.ClientRedis.UnLock(key)
		if unLock != 1 {
			glog.Errorf("FAILURE: UnLock [" + key + "]")
		}
	}

	if allowed {
		key := "ADMITEE_SMOOTH_DELETE_"+req.Namespace+"_"+req.Name
		vauleDelete, _ := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, key).Result()
		if vauleDelete == "" {
			value := "1"
			err = sm.ClientRedis.Client.SetNX(sm.ClientRedis.Ctx, key, value, 0).Err()
			if err == nil {
				glog.Infof("SUCCESS: SET [%s:%s]", key, value)
			}
		}
	}

	glog.Infof("MESSAGE: POD [%v], Delete [%v], Reason [%v]", req.Namespace+"/"+req.Name, allowed, reason)
	return returnAdmissionResponse(allowed, reason)
}


func (sm *SmoothManager) SmoothConfigExec(pod corev1.Pod) (bool, string) {
	smConfig, err := sm.GetSmoothConfig(pod)
	if smConfig == nil || err != nil {
		return false, err.Error()
	}

	var key = "ADMITEE_SMOOTH_POD_" + pod.Namespace + "_" + pod.Name
	vaulePOD, _ := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, key).Result()
	if vaulePOD == "" {
		_,err := sm.ClientKubeSet.CoreV1().Pods(pod.Namespace).Get(sm.Ctx, pod.Name, metav1.GetOptions{})
		if err == nil {
			var interval string
			if smConfig.Spec.Interval > 0 {
				interval = strconv.Itoa(smConfig.Spec.Interval)
			} else {
				interval = v1alpha1.DefaultInterval
			}

			value := pod.Namespace + "_" + pod.GetOwnerReferences()[0].Name + "_" + interval + "_" + strconv.FormatInt(time.Now().Unix(),10) + "_0"
			err := sm.ClientRedis.Client.SetNX(sm.ClientRedis.Ctx, key, value, 0).Err()
			if err == nil {
				glog.Infof("SUCCESS: SET [%s:%s]", key, value)
			}
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
			return false, fmt.Sprintf("FAILURE: Path NOT SET [%v]", rule)
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
				glog.Errorf("FAILURE: Body NOT SET [%v]", rule)
				return false, fmt.Sprintf("FAILURE: Body NOT SET [%v]", rule)
			}
			respStr, err = utils.RestApiPost(url, rule.Body)
		}

		if err != nil {
			reasons = append(reasons, "{"+ err.Error() +"}")
			glog.Infof("MESSAGE: Rule [%s] Reason [%v]", rule.Path, err)
		} else {
			reasons = append(reasons, "{"+rule.Method+" "+strconv.Itoa(rule.Port)+rule.Path+" "+respStr+"}")
			if respStr == strings.TrimSpace(rule.Expect) {
				glog.Infof("MESSAGE: Rule [%s] Reason [%s]", rule.Path, respStr)
			} else {
				allowed = false
			}
		}

		if !allowed {
			break
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
	list, err := sm.ClientKubeDynamic.Resource(gvr).Namespace(namespace).List(sm.Ctx, metav1.ListOptions{})
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

	return nil, fmt.Errorf("FAILURE: NO Smooth Matched [%s]", namespace + "/" + kindTarget + "/" + nameTarget)
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
			return "", "", fmt.Errorf("FAILURE: No Target Matched")
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
		glog.Errorf("FAILURE: POD KEYS [%s]: %v", reg, err)
	}

	// match pod by ownerReferenceName
	for _, v := range keys {
		var result string
		result, err = sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, v).Result()
		if err != nil {
			glog.Errorf("FAILURE: GET [%s]: %v", v, err)
			return countUpdate, err
		}
		podInfo := strings.Split(result,"_")
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
