package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	var kubeconfig *string

	if home := homedir.HomeDir(); home != "" {
		// 读取 $HOME/.kube/config 配置信息
		kubeconfig = flag.String(
			"kubeconfig",
			filepath.Join(home, ".kube", "config"),
			"(optional) absolute path to the kubeconfig file",
		)
	} else {
		// 参数 -kubeconfig=$HOME/.kube/config 指定配置信息
		kubeconfig = flag.String("kubeconfig",
			"",
			"absolute path to the kubeconfig file",
		)
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: toPtr(int32(2)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "demo",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "demo",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "web",
							Image: "nginx@latest",
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	fmt.Println("Creating deployment ...")
	res, err := deploymentsClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", res.GetObjectMeta().GetName())

	// Update Deployment
	prompt()
	fmt.Println("Updating deployment ...")
	//    You have two options to Update() this Deployment:
	//
	//    1. Modify the "deployment" variable and call: Update(deployment).
	//       This works like the "kubectl replace" command and it overwrites/loses changes
	//       made by other clients between you Create() and Update() the object.
	//    2. Modify the "result" returned by Get() and retry Update(result) until
	//       you no longer get a conflict error. This way, you can preserve changes made
	//       by other clients between Create() and Update(). This is implemented below
	//			 using the retry utility package included with client-go. (RECOMMENDED)
	//
	// More Info:
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency

	// 冲突重试
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		res, getErr := deploymentsClient.Get(context.TODO(), "demo-deployment", metav1.GetOptions{})
		if getErr != nil {
			panic(fmt.Errorf("Fail to get latest version of Depolyment: %v\n", getErr))
		}

		res.Spec.Replicas = toPtr(int32(1))
		res.Spec.Template.Spec.Containers[0].Image = "nginx"
		_, updateErr := deploymentsClient.Update(context.TODO(), res, metav1.UpdateOptions{})
		return updateErr
	})

	if retryErr != nil {
		panic(fmt.Errorf("Update failed: %v\n", retryErr))
	}
	fmt.Println("Updated deployment ...")

	// List Deployments
	prompt()
	fmt.Printf("Listing deployments in namespace: %q:\n", apiv1.NamespaceDefault)
	list, err := deploymentsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for _, d := range list.Items {
		fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
	}

	// Delete Deployment
	prompt()
	fmt.Println("Deleting deployment ...")
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete(context.TODO(), "demo-deployment", metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		panic(err)
	}

	fmt.Println("Deleted deployment.")
}

func prompt() {
	fmt.Printf("-> Press Return key to contine.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		break
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	fmt.Println()
}

func toPtr[T any](t T) *T {
	return &t
}
