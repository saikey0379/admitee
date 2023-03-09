package v1alpha1

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultInterval = 60
	DefaultTimeout  = 60
	DefaultPort     = 80
	DefaultMethod   = "get"
)

type Rule struct {
	Address string `json:"address"` // request address, default pod ip
	Port    int    `json:"port"`    // request port
	Path    string `json:"path"`    // request path
	Method  string `json:"method"`  // request method
	Body    string `json:"body"`    // request body for post method
	Expect  string `json:"expect"`  // expect response body
}

type SmoothSpec struct {
	// ScaleTargetRef is the reference to the workload that should be scaled.
	TargetRef autoscalingv2.CrossVersionObjectReference `json:"targetRef"`
	Rules     []Rule                                    `json:"rules"`
	Interval  int                                       `json:"interval"`
	Timeout   int                                       `json:"timeout"`
	SmLabel   string                                    `json:"smLabel"`
}

type Smooth struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec SmoothSpec `json:"spec,omitempty"`
}

type SmoothList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Smooth `json:"items"`
}
