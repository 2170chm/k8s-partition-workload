package controller

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	timeout          = 10 * time.Second
	testNSName       = v1.NamespaceDefault
	testPWName       = "test-pw"
	testUID          = types.UID("test")
	testOldImage     = "deprecated_nginx"
	testCurrentImage = "nginx"
	testUpdatedImage = "nginx2"
	testLabel        = map[string]string{"app": "test-app"}
	nilLabel         = map[string]string{}
)
