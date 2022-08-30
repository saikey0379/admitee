package utils 

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
)

func GetOwnerReference(pod corev1.Pod) (orKind string, orName string, err error) {
	orPod := pod.GetOwnerReferences()
	if len(orPod) > 1 {
		return "", "", fmt.Errorf("FAILURE: Too Many OwnerReference Matched")
	}
	if len(orPod) == 1 {
		return orPod[0].Kind, orPod[0].Name, err
	}
	return "", "", fmt.Errorf("FAILURE: No OwnerReference Matched")
}