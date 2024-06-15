package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/day253/healthcheck"
	"github.com/lwolf/kubereplay/helpers"
	"github.com/lwolf/kubereplay/pkg/apis/kubereplay/v1alpha1"
	"github.com/lwolf/kubereplay/pkg/client/clientset/versioned"
	"github.com/lwolf/kubereplay/pkg/client/informers/externalversions"
	kubereplayv1alpha1lister "github.com/lwolf/kubereplay/pkg/client/listers/kubereplay/v1alpha1"
	"k8s.io/api/apps/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultInitializerName = "kubereplay.initializer.lwolf.org"
)

var (
	initializerName string
	err             error
	clusterConfig   *rest.Config
	kubeconfig      string
)

var harvesterGVK = v1alpha1.SchemeGroupVersion.WithKind("Harvester")

func GenerateSidecar(refinerySvc string, hs v1alpha1.HarvesterSpec) *corev1.Container {
	var image string
	var imagePullPolicy apiv1.PullPolicy
	var resources corev1.ResourceRequirements

	if hs.Goreplay != nil && hs.Goreplay.Image != "" {
		image = hs.Goreplay.Image
	} else {
		image = "buger/goreplay:latest"
	}
	if hs.Goreplay != nil && hs.Goreplay.ImagePullPolicy != "" {
		imagePullPolicy = hs.Goreplay.ImagePullPolicy
	} else {
		imagePullPolicy = apiv1.PullAlways
	}
	if hs.Resources != nil {
		resources = *hs.Resources
	} else {
		resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		}
	}

	return &corev1.Container{
		Name:            "goreplay",
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Args: []string{
			"-input-raw",
			fmt.Sprintf(":%d", hs.AppPort),
			"-output-tcp",
			fmt.Sprintf("%s:28020", refinerySvc),
		},
		Resources: resources,
	}
}

func createShadowDeployment(d *v1beta1.Deployment, clientset *kubernetes.Clientset) {
	_, err = clientset.AppsV1beta1().Deployments(d.Namespace).Create(d)
	if err != nil {
		log.Printf("failed to create blue deployment %s: %v", d.Name, err)
	}

}

func initializeDeployment(deployment *v1beta1.Deployment, clientset *kubernetes.Clientset, lister kubereplayv1alpha1lister.HarvesterLister) error {
	if deployment.ObjectMeta.GetInitializers() != nil {
		pendingInitializers := deployment.ObjectMeta.GetInitializers().Pending
		if initializerName == pendingInitializers[0].Name {
			log.Printf("Initializing deployment: %s", deployment.Name)

			initializedDeploymentGreen := deployment.DeepCopy()

			// Remove self from the list of pending Initializers while preserving ordering.
			if len(pendingInitializers) == 1 {
				initializedDeploymentGreen.ObjectMeta.Initializers = nil
			} else {
				initializedDeploymentGreen.ObjectMeta.Initializers.Pending = append(pendingInitializers[:0], pendingInitializers[1:]...)
			}

			selector, err := metav1.LabelSelectorAsSelector(
				&metav1.LabelSelector{MatchLabels: deployment.ObjectMeta.GetLabels()},
			)
			var skip bool
			harvesters, err := lister.Harvesters(deployment.Namespace).List(labels.Everything())
			if err != nil {
				log.Printf("failed to get list of harvesters: %v", err)
				skip = true
			}
			var harvester *v1alpha1.Harvester
			for _, h := range harvesters {
				if labels.Equals(h.Spec.Selector, deployment.ObjectMeta.GetLabels()) {
					harvester = h
				}
			}

			if harvester == nil {
				log.Printf("debug: harvesters not found for deployment %s with selectors %v", deployment.Name, selector)
				skip = true
			}
			annotations := deployment.GetAnnotations()
			_, ok := annotations[helpers.AnnotationKeyDefault]
			if ok {
				skip = true
			}

			if skip {
				// Releasing original deployment
				_, err := clientset.AppsV1beta1().Deployments(deployment.Namespace).Update(initializedDeploymentGreen)
				if err != nil {
					log.Printf("failed to update initialized green deployment %s: %v ", initializedDeploymentGreen.Name, err)
					return err
				}
				return nil
			}

			initializedDeploymentBlue := helpers.CleanupDeployment(deployment)
			initializedDeploymentBlue.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(harvester, harvesterGVK),
			}
			initializedDeploymentBlue.ObjectMeta.Name = fmt.Sprintf("%s-gor", deployment.Name)

			//Remove self from the list of pending Initializers while preserving ordering.
			if len(pendingInitializers) == 1 {
				initializedDeploymentBlue.ObjectMeta.Initializers = nil
			} else {
				initializedDeploymentBlue.ObjectMeta.Initializers.Pending = append(pendingInitializers[:0], pendingInitializers[1:]...)
			}

			greenAnnotations := initializedDeploymentGreen.GetAnnotations()
			blueReplicas, greenReplicas := helpers.BlueGreenReplicas(*deployment.Spec.Replicas, int32(harvester.Spec.SegmentSize))
			if greenAnnotations == nil {
				greenAnnotations = make(map[string]string)
			}
			// set annotation for original deployment
			greenAnnotations[helpers.AnnotationKeyDefault] = helpers.AnnotationValueSkip
			greenAnnotations[helpers.AnnotationKeyReplicas] = strconv.Itoa(int(greenReplicas))
			greenAnnotations[helpers.AnnotationKeyShadow] = initializedDeploymentBlue.Name
			initializedDeploymentGreen.Annotations = greenAnnotations
			initializedDeploymentGreen.Spec.Replicas = &greenReplicas

			blueAnnotations := initializedDeploymentBlue.GetAnnotations()
			if blueAnnotations == nil {
				blueAnnotations = make(map[string]string)
			}
			blueAnnotations[helpers.AnnotationKeyDefault] = helpers.AnnotationValueCapture
			blueAnnotations[helpers.AnnotationKeyReplicas] = strconv.Itoa(int(blueReplicas))
			blueAnnotations[helpers.AnnotationKeyShadow] = initializedDeploymentGreen.Name
			initializedDeploymentBlue.Annotations = blueAnnotations
			initializedDeploymentBlue.Spec.Replicas = &blueReplicas
			initializedDeploymentBlue.Status = v1beta1.DeploymentStatus{}

			sidecar := GenerateSidecar(
				fmt.Sprintf("refinery-%s.%s", harvester.Spec.Refinery, harvester.Namespace),
				// todo: remove port from harvester spec, get it directly from deployment
				harvester.Spec,
			)

			_, err = clientset.AppsV1beta1().Deployments(deployment.Namespace).Update(initializedDeploymentGreen)
			if err != nil {
				log.Printf("failed to simply update initialized and updated green deployment %s: %v ", initializedDeploymentGreen.Name, err)
				return err
			}

			// Modify the Deployment's Pod template to include the Gor container
			if harvester.Spec.Goreplay != nil && harvester.Spec.Goreplay.ImagePullSecrets != nil {
				for _, ips := range harvester.Spec.Goreplay.ImagePullSecrets {
					initializedDeploymentBlue.Spec.Template.Spec.ImagePullSecrets = append(initializedDeploymentBlue.Spec.Template.Spec.ImagePullSecrets, ips)
				}
			}
			initializedDeploymentBlue.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *sidecar)
			// Creating new deployment in a go routine, otherwise it will block and timeout
			go createShadowDeployment(initializedDeploymentBlue, clientset)

		}
	}

	return nil
}

func main() {
	flag.StringVar(&initializerName, "initializer-name", defaultInitializerName, "The initializer name")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig")
	flag.Parse()
	log.Println("Starting the Kubernetes initializer...")
	log.Printf("Initializer name set to: %s", initializerName)

	if kubeconfig != "" {
		kubeConfigLocation := filepath.Join(kubeconfig)
		clusterConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigLocation)
	} else {
		clusterConfig, err = rest.InClusterConfig()
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatal(err)
	}
	health := healthcheck.NewHandler()
	health.AddReadinessCheck(
		"upstream-dns",
		healthcheck.DNSResolveCheck("kubernetes.default", 50*time.Millisecond))
	go http.ListenAndServe("0.0.0.0:8086", health)

	// Watch uninitialized Deployments in all namespaces.
	restClient := clientset.AppsV1beta1().RESTClient()
	watchlist := cache.NewListWatchFromClient(restClient, "deployments", corev1.NamespaceAll, fields.Everything())

	stop := make(chan struct{})
	cl := versioned.NewForConfigOrDie(clusterConfig)
	si := externalversions.NewSharedInformerFactory(cl, 30*time.Second)
	go si.Kubereplay().V1alpha1().Harvesters().Informer().Run(stop)
	si.WaitForCacheSync(stop)

	lister := si.Kubereplay().V1alpha1().Harvesters().Lister()

	// Wrap the returned watchlist to workaround the inability to include
	// the `IncludeUninitialized` list option when setting up watch clients.
	includeUninitializedWatchlist := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.IncludeUninitialized = true
			return watchlist.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.IncludeUninitialized = true
			return watchlist.Watch(options)
		},
	}

	resyncPeriod := 30 * time.Second

	_, controller := cache.NewInformer(includeUninitializedWatchlist, &v1beta1.Deployment{}, resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				err := initializeDeployment(obj.(*v1beta1.Deployment), clientset, lister)
				if err != nil {
					log.Println(err)
				}
			},
		},
	)
	go controller.Run(stop)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting...")
	close(stop)
}
