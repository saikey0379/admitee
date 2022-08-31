package smooth

import (
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
	if deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal != "0" {
		maxuvfloat64, err := strconv.ParseFloat(strings.Replace(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal, "%", "", -1), 64)
		if err != nil {
			glog.Errorf("FAILURE: Can't encode maxuvfloat64[%v]", err)
			return false, "Deployment MaxUnavailable[" + err.Error() + "]"
		}
		maxuvpercent := maxuvfloat64 * 0.01
		countMaxuav = int(float64(*deployment.Spec.Replicas) * maxuvpercent)
		if countMaxuav == 0 {
			countMaxuav = 1
		}
		glog.Infof("MESSAGE: Deployment[%s] SmoothCount: %v, MaxUnavailableCount: %v, DesiredNumber：%v, MaxUnavailable:%v", project, countUpdate, countMaxuav, *deployment.Spec.Replicas, deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal)

		//删除副本数大于等于最大不可用副本数时，拒绝删除
		if countUpdate >= countMaxuav {
			return false, "Deployment exceed maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
		}
	}
	return true, "Deployment maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
}
