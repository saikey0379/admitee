package smooth

import (
	"math"
	"strconv"
	"strings"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sm *SmoothManager) VerifyDeletePodReplicaSet(namespace string, rsName string, countUpdate int) (bool, string) {
	var countMaxuav int

	// get replicaset countMaxuav by Desired Replicas
	replicaset, err := sm.ClientKubeSet.AppsV1().ReplicaSets(namespace).Get(sm.Ctx, rsName, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("FAILURE: Get ReplicaSet[%v]", err)
		return false, "ReplicaSet GET[" + err.Error() + "]"
	}

	countMaxuav = int(replicaset.Status.Replicas - *replicaset.Spec.Replicas)
	if countMaxuav > 0 {
		glog.Infof("MESSAGE: ReplicaSet[%s] SmoothCount[%v] MaxUnavailableCount[%v]", rsName, countUpdate, countMaxuav)
	} else {
		// get replicaset countMaxuav by deployment MaxUnavailable Replicas
		dpKind := replicaset.GetOwnerReferences()[0].Kind
		dpName := replicaset.GetOwnerReferences()[0].Name
		if dpKind == "Deployment" {
			deployment, err := sm.ClientKubeSet.AppsV1().Deployments(namespace).Get(sm.Ctx, dpName, metav1.GetOptions{})
			if err != nil {
				glog.Errorf("FAILURE: Get Deployment[%v]", err)
			} else {
				if deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntValue() != 0 {
					countMaxuav = deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntValue()
				} else {
					maxuvfloat64, err := strconv.ParseFloat(strings.Replace(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.String(), "%", "", -1), 64)
					if err != nil {
						glog.Errorf("FAILURE: Can't encode Deployment MaxUnavailable float64[%v]", err)
					}
					countMaxuav = int(float64(*deployment.Spec.Replicas) * maxuvfloat64 * 0.01)
				}
				glog.Infof("MESSAGE: Deployment[%s] SmoothCount[%v] MaxUnavailableCount[%v] Replicas[%v] MaxUnavailable[%v]", dpName, countUpdate, countMaxuav, *deployment.Spec.Replicas, deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.String())
				// get replicaset countMaxuav by deployment MaxSurge Replicas
				if countMaxuav == 0 {
					if deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntValue() != 0 {
						countMaxuav = deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntValue()
					} else {
						maxuvfloat64, err := strconv.ParseFloat(strings.Replace(deployment.Spec.Strategy.RollingUpdate.MaxSurge.String(), "%", "", -1), 64)
						if err != nil {
							glog.Errorf("FAILURE: Can't encode Deployment MaxSurge float64[%v]", err)
						}
						countMaxuav = int(math.Ceil(float64(*deployment.Spec.Replicas) * maxuvfloat64 * 0.01))
					}
					glog.Infof("MESSAGE: Deployment[%s] SmoothCount[%v] MaxUnavailableCount[%v] Replicas[%v] MaxSurge[%v]", dpName, countUpdate, countMaxuav, *deployment.Spec.Replicas, deployment.Spec.Strategy.RollingUpdate.MaxSurge.String())
				}
			}
		}
	}

	if countMaxuav == 0 {
		countMaxuav = 1
	}

	//删除副本数大于等于最大不可用副本数时，拒绝删除
	if countUpdate >= countMaxuav {
		return false, "ReplicaSet exceed maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
	}
	return true, "ReplicaSet maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
}
