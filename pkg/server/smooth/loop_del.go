package smooth

import (
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sm *SmoothManager) LoopSmooth() {
	var reg = "ADMITEE_SMOOTH_POD_*"
	for {
		key := "ADMITEE_SMOOTH_LOCK_LOOP_POD"
		for {
			boolLock := sm.ClientRedis.Lock(key)
			if !boolLock {
				time.Sleep(time.Duration(1) * time.Second)
				continue
			} else {
				break
			}
		}

		keyPODs, err := sm.ClientRedis.Client.Keys(sm.ClientRedis.Ctx, reg).Result()
		if err != nil {
			glog.Errorf("FAILURE: KEYS[%s]: %v", reg, err)
		}
		for _, keyPOD := range keyPODs {
			valuePOD, err := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, keyPOD).Result()
			if err != nil {
				glog.Errorf("FAILURE: GET[%s]: %v", keyPOD, err)
				continue
			}
			keyInfo := strings.Split(keyPOD, "_")
			namespace := keyInfo[3]
			podName := keyInfo[4]

			valueInfo := strings.Split(valuePOD, "_")
			interval, _ := strconv.Atoi(valueInfo[2])
			lastime, _ := strconv.Atoi(valueInfo[3])
			count, _ := strconv.Atoi(valueInfo[4])
			if lastime+interval <= int(time.Now().Unix()) {
				var errDEL error
				_, errGET := sm.ClientKubeSet.CoreV1().Pods(namespace).Get(sm.ClientRedis.Ctx, podName, metav1.GetOptions{})
				if errGET == nil {
					//POD存在，则删除POD
					errDEL = sm.ClientKubeSet.CoreV1().Pods(namespace).Delete(sm.ClientRedis.Ctx, podName, metav1.DeleteOptions{})
					if errDEL != nil {
						//删除失败，更新RDB
						valueInfo[3] = strconv.FormatInt(time.Now().Unix(), 10)
						valueInfo[4] = strconv.Itoa(count + 1)
						value := strings.Join(valueInfo, "_")
						//更新Redis
						err = sm.ClientRedis.Client.Set(sm.ClientRedis.Ctx, keyPOD, value, 0).Err()
						if err != nil {
							glog.Errorf("FAILURE: SET[%s:%s]: %v", keyPOD, value, err)
						} else {
							glog.Infof("SUCCESS: SET[%v:%v]", keyPOD, value)
						}
					}
				}

				if errGET != nil || (errGET == nil && errDEL == nil) || count*interval >= 3600 {
					n, _ := sm.ClientRedis.Client.Exists(sm.ClientRedis.Ctx, "ADMITEE_SMOOTH_DEL_"+namespace+"_"+podName).Result()
					if n == 0 {
						//删除RDB记录
						err := sm.ClientRedis.Client.Del(sm.ClientRedis.Ctx, keyPOD).Err()
						if err != nil {
							glog.Errorf("FAILURE: DEL[%s]: %v", keyPOD, err)
						} else {
							glog.Infof("SUCCESS: DEL[%s]", keyPOD)
						}
					}
				}
			}
		}
		result := sm.ClientRedis.UnLock(key)
		if result != 1 {
			glog.Errorf("FAILURE: UNLOCK [ADMITEE_SMOOTH_LOCK_LOOP_POD]")
		}
		time.Sleep(time.Duration(10) * time.Second)
	}
}

func (sm *SmoothManager) LoopDelete() {
	var reg = "ADMITEE_SMOOTH_DEL_*"
	for {
		key := "ADMITEE_SMOOTH_LOCK_LOOP_DELETE"
		for {
			boolLock := sm.ClientRedis.Lock(key)
			if !boolLock {
				time.Sleep(time.Duration(1) * time.Second)
				continue
			} else {
				break
			}
		}

		keyDELETEs, err := sm.ClientRedis.Client.Keys(sm.ClientRedis.Ctx, reg).Result()
		if err != nil {
			glog.Errorf("FAILURE: KEYS[%s]: %v", reg, err)
		}
		for _, keyDELETE := range keyDELETEs {
			_, err := sm.ClientRedis.Client.Get(sm.ClientRedis.Ctx, keyDELETE).Result()
			if err != nil {
				glog.Errorf("FAILURE: GET[%s]: %v", keyDELETE, err)
				continue
			}

			keyInfo := strings.Split(keyDELETE, "_")
			namespace := keyInfo[3]
			podName := keyInfo[4]

			keyPOD := "ADMITEE_SMOOTH_POD_" + namespace + "_" + podName
			_, err = sm.ClientKubeSet.CoreV1().Pods(namespace).Get(sm.ClientRedis.Ctx, podName, metav1.GetOptions{})
			if err != nil {
				//POD不存在，删除key，避免轮询更新冲突，加锁
				key := "ADMITEE_SMOOTH_LOCK_LOOP_POD"
				for {
					boolLock := sm.ClientRedis.Lock(key)
					if !boolLock {
						time.Sleep(time.Duration(1) * time.Second)
						continue
					} else {
						break
					}
				}

				err = sm.ClientRedis.Client.Del(sm.ClientRedis.Ctx, keyPOD).Err()
				if err != nil {
					glog.Errorf("FAILURE: DEL[%s]: %v", keyPOD, err)
				} else {
					glog.Infof("SUCCESS: DEL[%s]", keyPOD)
				}

				err = sm.ClientRedis.Client.Del(sm.ClientRedis.Ctx, keyDELETE).Err()
				if err != nil {
					glog.Errorf("FAILURE: DEL[%s]: %v", keyDELETE, err)
				} else {
					glog.Infof("SUCCESS: DEL[%s]", keyDELETE)
				}

				result := sm.ClientRedis.UnLock(key)
				if result != 1 {
					glog.Errorf("FAILURE: UNLOCK [ADMITEE_SMOOTH_LOCK_LOOP_POD]")
				}

				time.Sleep(time.Duration(1) * time.Second)
			}
		}
		time.Sleep(time.Duration(1) * time.Second)
		sm.ClientRedis.UnLock(key)
	}
}
