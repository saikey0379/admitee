package smooth

import (
	"strconv"
	"strings"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sm *SmoothManager) VerifyDeletePodDaemonSet(namespace string, project string, countUpdate int) (bool, string) {
	dsdetail, err := sm.ClientKubeSet.AppsV1().DaemonSets(namespace).Get(sm.Ctx, project, metav1.GetOptions{})
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
	glog.Infof("MESSAGE: DaemonSet[%s] MaxUnavailableCount: %v，DesiredNumber：%v, MaxUnavailable:%v", project, countMaxuav, dsdetail.Status.DesiredNumberScheduled, dsdetail.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.StrVal)

	//无可用副本，允许删除
	if dsdetail.Status.NumberAvailable == 0 {
		return true, "DaemonSet NumberAvailable[" + strconv.Itoa(int(dsdetail.Status.NumberAvailable)) + "]"
	}

	//单副本，允许删除
	if dsdetail.Status.DesiredNumberScheduled == 1 {
		return true, "DaemonSet DesiredNumberScheduled[" + strconv.Itoa(int(dsdetail.Status.DesiredNumberScheduled)) + "]"
	}

	//删除副本数大于等于最大不可用副本数时，拒绝删除
	if countUpdate >= countMaxuav {
		return false, "DaemonSet exceed maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
	}

	return true, "DaemonSet maxUnavailable[" + strconv.Itoa(countUpdate) + "/" + strconv.Itoa(countMaxuav) + "]"
}
