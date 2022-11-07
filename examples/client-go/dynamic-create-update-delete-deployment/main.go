package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
	"os"
	"path/filepath"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	deployRes := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "demo-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": 2,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "demo",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "demo",
						},
					},
					"spec": map[string]interface{}{
						"containers": []map[string]interface{}{
							{
								"name":  "web",
								"image": "nginx",
								"ports": []map[string]interface{}{
									{
										"name":          "http",
										"protocol":      "TCP",
										"containerPort": 80,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Create Deployment
	fmt.Println("Creating deployment ...")
	res, err := client.Resource(deployRes).Namespace(apiv1.NamespaceDefault).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", res.GetName())

	// Update deployment
	prompt()
	fmt.Println("Updating deployment ...")

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		res, err := client.Resource(deployRes).Namespace(apiv1.NamespaceDefault).Get(context.TODO(), "demo-deployment", metav1.GetOptions{})
		if err != nil {
			panic(fmt.Errorf("failed to get latest version of Deployment: %v\n", err))
		}

		if err := unstructured.SetNestedField(res.Object, int64(1), "spec", "replicas"); err != nil {
			panic(fmt.Errorf("failed to set replica value: %v\n", err))
		}

		containers, found, err := unstructured.NestedSlice(res.Object, "spec", "template", "spec", "containers")
		if err != nil || !found || containers == nil {
			panic(fmt.Errorf("depolyment containers not found or error in spec: %v\n", err))
		}

		if err := unstructured.SetNestedField(containers[0].(map[string]interface{}), "nginx:1.13", "image"); err != nil {
			panic(err)
		}
		if err := unstructured.SetNestedField(res.Object, containers, "spec", "template", "spec", "containers"); err != nil {
			panic(err)
		}

		_, updateErr := client.Resource(deployRes).Namespace(apiv1.NamespaceDefault).Update(context.TODO(), res, metav1.UpdateOptions{})
		return updateErr
	})

	if retryErr != nil {
		panic(fmt.Errorf("update failed: %v\n", retryErr))
	}
	fmt.Println("Updated deployment ...")

	// List Deployments
	prompt()
	fmt.Printf("Listing deployments in namespace %q:\n", apiv1.NamespaceDefault)
	list, err := client.Resource(deployRes).Namespace(apiv1.NamespaceDefault).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	for _, d := range list.Items {
		replicas, found, err := unstructured.NestedInt64(d.Object, "spec", "replicas")
		if err != nil || !found {
			fmt.Printf("Replicas not found for deployment %s: error=%s", d.GetName(), err)
		}
		fmt.Printf(" * %s (%d replicas)\n", d.GetName(), replicas)
	}

	// Delete Deployment
	prompt()
	fmt.Println("Deleting deployment ...")
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}
	if err := client.Resource(deployRes).Namespace(apiv1.NamespaceDefault).Delete(context.TODO(), "demo-deployment", deleteOptions); err != nil {
		panic(err)
	}

	fmt.Println("Deleted deployment ...")
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
