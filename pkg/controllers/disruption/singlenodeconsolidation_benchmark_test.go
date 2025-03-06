/*
Copyright The Kubernetes Authors.

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

package disruption_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
	"github.com/samber/lo"

	//appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/controllers/disruption"
	pscheduling "sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("ConsolidationPerformance", func() {
	var nodePool *v1.NodePool
	//var nodeClaim *v1.NodeClaim
	//var numNodes = 1000
	var nodeClaims []*v1.NodeClaim
	var nodes []*corev1.Node
	//var rs *appsv1.ReplicaSet
	//var labels = map[string]string{
	//	"app": "test",
	//}
	var ctx context.Context
	BeforeEach(func() {
		ctx = options.ToContext(context.Background(), test.Options(test.OptionsFields{FeatureGates: test.FeatureGates{SpotToSpotConsolidation: lo.ToPtr(false)}}))
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Disruption: v1.Disruption{
					ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
					// Disrupt away!
					Budgets: []v1.Budget{{
						Nodes: "100%",
					}},
					ConsolidateAfter: v1.MustParseNillableDuration("0s"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool)
		/*
			nodeClaim, _ = test.NodeClaimAndNode(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("32")},
				},
			})
			nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			nodeClaims, nodes = test.NodeClaimsAndNodes(numNodes, v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.NodePoolLabelKey:            nodePool.Name,
						corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
						v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
						corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
					},
				},
				Status: v1.NodeClaimStatus{
					Allocatable: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:  resource.MustParse("32"),
						corev1.ResourcePods: resource.MustParse("100"),
					},
				},
			})
			for _, nc := range nodeClaims {
				nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
			}


		*/
	})
	Context("SingleNodeConsolidation", func() {
		It("Should perform single node consolidation within XXXX ms", func() {
			experiment := gmeasure.NewExperiment("SingleNode Consolidation")
			AddReportEntry(experiment.Name, experiment)
			var singleConsolidation *disruption.SingleNodeConsolidation
			var budgets map[string]int
			var err error
			var candidates []*disruption.Candidate
			var cmd disruption.Command
			var results pscheduling.Results
			experiment.Sample(func(idx int) {
				nodeClaims = make([]*v1.NodeClaim, 0, 1)
				nodes = make([]*corev1.Node, 0, 1)
				// Create 3 nodes for each nodePool
				ncs, ns := test.NodeClaimsAndNodes(2, v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey:            nodePool.Name,
							corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
							v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
							corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
						},
					},
					Status: v1.NodeClaimStatus{
						Allocatable: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:  resource.MustParse("32"),
							corev1.ResourcePods: resource.MustParse("100"),
						},
					},
				})
				nodeClaims = append(nodeClaims, ncs...)
				nodes = append(nodes, ns...)
				fmt.Println(len(nodeClaims))
				fmt.Println(len(nodes))
				// create our RS so we can link a pod to it
				rs := test.ReplicaSet()
				ExpectApplied(ctx, env.Client, rs)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())
				ExpectApplied(ctx, env.Client, nodePool)
				nodeClaims[0].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
				nodeClaims[1].StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
				pod := test.Pod()
				ExpectApplied(ctx, env.Client, pod, nodes[0], nodeClaims[0])
				ExpectApplied(ctx, env.Client, nodes[1], nodeClaims[1])
				ExpectManualBinding(ctx, env.Client, pod, nodes[0])
				// inform cluster state about nodes and nodeclaims
				ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)

				experiment.MeasureDuration("singlenodeconsolidation", func() {
					singleConsolidation = disruption.NewSingleNodeConsolidation(disruption.MakeConsolidation(fakeClock, cluster, env.Client, prov, cloudProvider, recorder, queue))
					fmt.Println(singleConsolidation)
					budgets, err = disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, singleConsolidation.Reason())
					Expect(err).To(Succeed())
					fmt.Println(budgets)

					candidates, err = disruption.GetCandidates(ctx, cluster, env.Client, recorder, fakeClock, cloudProvider, singleConsolidation.ShouldDisrupt, singleConsolidation.Class(), queue)
					Expect(err).To(Succeed())
					fmt.Println(candidates)

					cmd, results, err = singleConsolidation.ComputeCommand(ctx, budgets, candidates...)
					Expect(err).To(Succeed())
					Expect(results).To(Equal(pscheduling.Results{}))
					Expect(cmd).To(Equal(disruption.Command{}))
					Expect(singleConsolidation.IsConsolidated()).To(BeTrue())
					fmt.Println(cmd)
					fmt.Println(results)
				}, gmeasure.Precision(time.Millisecond))
			}, gmeasure.SamplingConfig{N: 1})
		})
	})
})
