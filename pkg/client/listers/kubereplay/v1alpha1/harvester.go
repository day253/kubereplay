/*
Copyright 2017 Sergey Nuzhdin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	"github.com/lwolf/kubereplay/pkg/apis/kubereplay/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// HarvesterLister helps list Harvesters.
type HarvesterLister interface {
	// List lists all Harvesters in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.Harvester, err error)
	// Harvesters returns an object that can list and get Harvesters.
	Harvesters(namespace string) HarvesterNamespaceLister
	HarvesterListerExpansion
}

// harvesterLister implements the HarvesterLister interface.
type harvesterLister struct {
	indexer cache.Indexer
}

// NewHarvesterLister returns a new HarvesterLister.
func NewHarvesterLister(indexer cache.Indexer) HarvesterLister {
	return &harvesterLister{indexer: indexer}
}

// List lists all Harvesters in the indexer.
func (s *harvesterLister) List(selector labels.Selector) (ret []*v1alpha1.Harvester, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Harvester))
	})
	return ret, err
}

// Harvesters returns an object that can list and get Harvesters.
func (s *harvesterLister) Harvesters(namespace string) HarvesterNamespaceLister {
	return harvesterNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// HarvesterNamespaceLister helps list and get Harvesters.
type HarvesterNamespaceLister interface {
	// List lists all Harvesters in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.Harvester, err error)
	// Get retrieves the Harvester from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.Harvester, error)
	HarvesterNamespaceListerExpansion
}

// harvesterNamespaceLister implements the HarvesterNamespaceLister
// interface.
type harvesterNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Harvesters in the indexer for a given namespace.
func (s harvesterNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.Harvester, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Harvester))
	})
	return ret, err
}

// Get retrieves the Harvester from the indexer for a given namespace and name.
func (s harvesterNamespaceLister) Get(name string) (*v1alpha1.Harvester, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("harvester"), name)
	}
	return obj.(*v1alpha1.Harvester), nil
}
