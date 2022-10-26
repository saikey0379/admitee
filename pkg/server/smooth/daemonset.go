package smooth

import (
	"strconv"
	"strings"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sm *SmoothManager) VerifyDeletePodDaemonSet(namespace string, dsName string, countUpdate int) (bool, string) {
	dsdetail, err := sm.ClientKubeSet.AppsV1().DaemonSets(namespace).Get(sm.Ctx, dsName, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("FAILURE: Get DaemonSet[%v]", err)
		return false, "DaemonSet GET[" + err.Error() + "]"
	}

	maxuvfloat64, err := strconv.ParseFloat(strings.Replace(dsdetail.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.StrVal, "%", "", -1), 64)
	if err != nil {
		glog.Errorf("FAILURE: Can't encode maxuvfloat64[%v]", err)
		return false, "DaemonSet MaxUnavailable[" + err.Error() + "]"
	}
	maxuvpercent := maxuvfloat64 * 0.01
	countMaxuav := int(float64(dsdetail.Status.DesiredNumberScheduled) * maxuvpercent)
	if countMaxuav == 0 {
		countMaxuav = 1
	}
	glog.Infof("MESSAGE: DaemonSet[%s] SmoothCount: %v, MaxUnavailableCount: %v, DesiredNumber：%v, MaxUnavailable:%v", dsName, countUpdate, countMaxuav, dsdetail.Status.DesiredNumberScheduled, dsdetail.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.StrVal)

	//删除副本数大于等于最大不可用副本数时，拒绝删除
	if countUpdate >= countMaxuav {
		return false, "DaemonSet exceed maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
	}

	return true, "DaemonSet maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
}
