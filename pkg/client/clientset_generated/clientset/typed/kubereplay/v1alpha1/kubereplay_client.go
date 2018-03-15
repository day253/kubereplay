package v1alpha1

import (
	"github.com/lwolf/kubereplay/pkg/apis/kubereplay/v1alpha1"
	"github.com/lwolf/kubereplay/pkg/client/clientset_generated/clientset/scheme"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

type KubereplayV1alpha1Interface interface {
	RESTClient() rest.Interface
	HarvestersGetter
	RefineriesGetter
}

// KubereplayV1alpha1Client is used to interact with features provided by the kubereplay.lwolf.org group.
type KubereplayV1alpha1Client struct {
	restClient rest.Interface
}

func (c *KubereplayV1alpha1Client) Harvesters(namespace string) HarvesterInterface {
	return newHarvesters(c, namespace)
}

func (c *KubereplayV1alpha1Client) Refineries(namespace string) RefineryInterface {
	return newRefineries(c, namespace)
}

// NewForConfig creates a new KubereplayV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*KubereplayV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &KubereplayV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new KubereplayV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *KubereplayV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new KubereplayV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *KubereplayV1alpha1Client {
	return &KubereplayV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *KubereplayV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}