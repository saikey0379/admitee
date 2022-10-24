package smooth

import (
	"math"
	"strconv"
	"strings"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sm *SmoothManager) VerifyDeletePodDeployment(namespace string, project string, countUpdate int) (bool, string) {
	deployment, err := sm.ClientKubeSet.AppsV1().Deployments(namespace).Get(sm.Ctx, project, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("FAILURE: Get DaemonSet[%v]", err)
		return false, "Deployment GET[" + err.Error() + "]"
	}

	//无可用副本，允许删除
	if deployment.Status.AvailableReplicas == 0 {
		return true, "Deployment AvailableReplicas[" + strconv.Itoa(int(deployment.Status.AvailableReplicas)) + "]"
	}

	if int(*deployment.Spec.Replicas)-countUpdate <= 1 {
		return false, "Deployment Replicas/Smoothing[" + strconv.Itoa(int(*deployment.Spec.Replicas)) + "/" + strconv.Itoa(countUpdate) + "]"
	}

	var countMaxuav int
	// get countMaxuav by MaxUnavailable
	if deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntValue() != 0 {
		countMaxuav = deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntValue()
	} else {
		maxuvfloat64, err := strconv.ParseFloat(strings.Replace(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.String(), "%", "", -1), 64)
		if err != nil {
			glog.Errorf("FAILURE: Can't encode maxuvfloat64[%v]", err)
		}
		countMaxuav = int(float64(*deployment.Spec.Replicas) * maxuvfloat64 * 0.01)
	}
	glog.Infof("MESSAGE: Deployment[%s] SmoothCount: %v, MaxUnavailableCount: %v, DesiredNumber：%v, MaxUnavailable:%v", project, countUpdate, countMaxuav, *deployment.Spec.Replicas, deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.String())

	// get countMaxuav by MaxSurge
	if countMaxuav == 0 {
		if deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntValue() != 0 {
			countMaxuav = deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntValue()
		} else {
			maxuvfloat64, err := strconv.ParseFloat(strings.Replace(deployment.Spec.Strategy.RollingUpdate.MaxSurge.String(), "%", "", -1), 64)
			if err != nil {
				glog.Errorf("FAILURE: Can't encode maxuvfloat64[%v]", err)
			}
			countMaxuav = int(math.Ceil(float64(*deployment.Spec.Replicas) * maxuvfloat64 * 0.01))
		}
		glog.Infof("MESSAGE: Deployment[%s] SmoothCount: %v, MaxUnavailableCount: %v, DesiredNumber：%v, MaxSurge:%v", project, countUpdate, countMaxuav, *deployment.Spec.Replicas, deployment.Spec.Strategy.RollingUpdate.MaxSurge.String())
	}

	if countMaxuav == 0 {
		countMaxuav = 1
	}

	//删除副本数大于等于最大不可用副本数时，拒绝删除
	if countUpdate >= countMaxuav {
		return false, "Deployment exceed maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
	}
	return true, "Deployment maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
}
