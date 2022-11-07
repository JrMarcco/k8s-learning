package main

import (
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"testing"
	"time"
)

func TestFakeClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcherStarted := make(chan struct{})
	// Create the fake client.
	client := fake.NewSimpleClientset()
	// A catch-all watch reactor that allows us to inject the watcherStarted channel.
	client.PrependWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watcher, err := client.Tracker().Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		close(watcherStarted)
		return true, watcher, nil
	})

	// we will create an informer that writes added pods to a channel
	pods := make(chan *v1.Pod, 1)
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	podInformer := informerFactory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			t.Logf("pod added: %s/%s", pod.Namespace, pod.Namespace)
			pods <- pod
		},
	})

	// Make sure informerFactory are running
	informerFactory.Start(ctx.Done())

	cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced)

	<-watcherStarted

	p := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "my-pod"}}
	_, err := client.CoreV1().Pods("test-ns").Create(context.TODO(), p, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("error injecting pod add: %v\n", err)
	}

	select {
	case pod := <-pods:
		t.Logf("Got pod from channel: %s/%s\n", pod.Namespace, pod.Name)
	case <-time.After(wait.ForeverTestTimeout):
		t.Errorf("Informer did not the added pod\n")
	}
}
