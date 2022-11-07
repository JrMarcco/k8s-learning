package kubernetes

import (
	"errors"
	"flag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

type K8sClient struct {
	kubeconfig *rest.Config
	clientset  *kubernetes.Clientset
}

func NewK8sClient() (*K8sClient, error) {
	var kubeconfig *string

	if home := homedir.HomeDir(); home != "" {
		// 读取 $HOME/.kube/config 配置信息
		kubeconfig = flag.String(
			"kubeconfig",
			filepath.Join(home, ".kube", "config"),
			"",
		)
	} else {
		return nil, errors.New("can not find kube config file")
	}

	// use the current context in kubeconfig

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8sClient{
		kubeconfig: config,
		clientset:  clientset,
	}, nil
}
