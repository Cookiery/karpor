// Copyright The Karpor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package insight

import (
	"context"
	"strings"

	"github.com/KusionStack/karpor/pkg/core/entity"
	"github.com/KusionStack/karpor/pkg/core/handler"
	"github.com/KusionStack/karpor/pkg/infra/multicluster"
	topologyutil "github.com/KusionStack/karpor/pkg/util/topology"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "sigs.k8s.io/yaml"
)

// getResource gets the resource from the cluster or storage.
func (i *InsightManager) getResource(
	ctx context.Context, client *multicluster.MultiClusterClient, resourceGroup *entity.ResourceGroup,
) (*unstructured.Unstructured, error) {
	resourceGVR, err := topologyutil.GetGVRFromGVK(resourceGroup.APIVersion, resourceGroup.Kind)
	if err != nil {
		return nil, err
	}
	resource, err := client.DynamicClient.Resource(resourceGVR).Namespace(resourceGroup.Namespace).Get(ctx, resourceGroup.Name, metav1.GetOptions{})

	if err != nil && k8serrors.IsNotFound(err) {
		if r, err := i.search.SearchByTerms(ctx, resourceGroup.ToTerms(), nil); err == nil && len(r.Resources) > 0 {
			resource = &unstructured.Unstructured{}
			resource.SetUnstructuredContent(r.Resources[0].Object)
			return resource, nil
		}
	}

	return resource, err
}

// GetResource returns the unstructured cluster object for a given cluster.
func (i *InsightManager) GetResource(
	ctx context.Context, client *multicluster.MultiClusterClient, resourceGroup *entity.ResourceGroup,
) (*unstructured.Unstructured, error) {
	resource, err := i.getResource(ctx, client, resourceGroup)
	if err != nil {
		return nil, err
	}
	resource, err = handler.RemoveUnstructuredManagedFields(ctx, resource)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(resourceGroup.Kind, "Secret") {
		return i.SanitizeSecret(resource)
	}
	return resource, err
}

// GetYAMLForResource returns the yaml byte array for a given cluster
func (i *InsightManager) GetYAMLForResource(
	ctx context.Context, client *multicluster.MultiClusterClient, resourceGroup *entity.ResourceGroup,
) ([]byte, error) {
	obj, err := i.GetResource(ctx, client, resourceGroup)
	if err != nil {
		return nil, err
	}
	return k8syaml.Marshal(obj.Object)
}

// SanitizeSecret redact the data field in the secret object
func (i *InsightManager) SanitizeSecret(original *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	sanitized := original
	if _, ok := sanitized.Object["data"]; ok {
		sanitized.Object["data"] = "[redacted]"
	}
	return original, nil
}
