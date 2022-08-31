package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"admitee/pkg/server/smooth"

	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter = runtime.ObjectDefaulter(runtimeScheme)
	podconf   = make(map[string]bool)
)

func (s *apiServer) Admission(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = s.ReturnAdmissionResponse(r.URL.Path, &ar)
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	//	glog.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func (s *apiServer) ReturnAdmissionResponse(url string, ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	var admissionResp *v1beta1.AdmissionResponse
	switch url {
	case "/admission/smooth":
		var sm = &smooth.SmoothManager{
			ClientKubeDynamic: s.clientKubeDynamic,
			ClientKubeSet:     s.clientKubeSet,
			ClientRedis:       s.clientRedis,
			Ctx:               context.Background(),
		}
		return sm.EnterSmoothProcess(ar)
	}
	return admissionResp
}

func (s *apiServer) DeamonSmooth() {
	var sm = &smooth.SmoothManager{
		ClientKubeSet: s.clientKubeSet,
		ClientRedis:   s.clientRedis,
		Ctx:           context.Background(),
	}
	go sm.LoopSmooth()
	go sm.LoopDelete()
}
