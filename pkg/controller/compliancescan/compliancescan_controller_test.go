package compliancescan

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	complianceoperatorv1alpha1 "github.com/openshift/compliance-operator/pkg/apis/complianceoperator/v1alpha1"
)

var _ = Describe("Testing compliancescan controller phases", func() {

	var (
		compliancescaninstance *complianceoperatorv1alpha1.ComplianceScan
		reconciler             ReconcileComplianceScan
		logger                 logr.Logger
		nodeinstance1          *corev1.Node
		nodeinstance2          *corev1.Node
	)

	BeforeEach(func() {
		logger = zapr.NewLogger(zap.NewNop())
		objs := []runtime.Object{}

		// test instance
		compliancescaninstance = &complianceoperatorv1alpha1.ComplianceScan{}
		objs = append(objs, compliancescaninstance)

		// Nodes in the deployment
		nodeinstance1 = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
		}
		nodeinstance2 = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
		}
		objs = append(objs, nodeinstance1, nodeinstance2)
		scheme := scheme.Scheme
		scheme.AddKnownTypes(complianceoperatorv1alpha1.SchemeGroupVersion, compliancescaninstance)

		client := fake.NewFakeClientWithScheme(scheme, objs...)
		reconciler = ReconcileComplianceScan{client: client, scheme: scheme}
	})

	Context("On the PENDING phase", func() {
		It("should update the compliancescan instance to phase LAUNCHING", func() {
			result, err := reconciler.phasePendingHandler(compliancescaninstance, logger)
			Expect(result).NotTo(BeNil())
			Expect(err).To(BeNil())
			Expect(compliancescaninstance.Status.Phase).To(Equal(complianceoperatorv1alpha1.PhaseLaunching))
		})
	})

	Context("On the LAUNCHING phase", func() {
		It("should update the compliancescan instance to phase RUNNING", func() {
			result, err := reconciler.phaseLaunchingHandler(compliancescaninstance, logger)
			Expect(result).ToNot(BeNil())
			Expect(err).To(BeNil())
			Expect(compliancescaninstance.Status.Phase).To(Equal(complianceoperatorv1alpha1.PhaseRunning))

			// We should have scheduled a pod per node
			nodes, _ := getTargetNodes(&reconciler, compliancescaninstance)
			var pods corev1.PodList
			reconciler.client.List(context.TODO(), &pods)
			Expect(len(pods.Items)).To(Equal(len(nodes.Items)))
		})
	})

	Context("On the RUNNING phase", func() {
		Context("With no pods in the cluster", func() {
			It("should update the compliancescan instance to phase DONE", func() {
				result, err := reconciler.phaseRunningHandler(compliancescaninstance, logger)
				Expect(result).ToNot(BeNil())
				Expect(err).To(BeNil())
				Expect(compliancescaninstance.Status.Phase).To(Equal(complianceoperatorv1alpha1.PhaseDone))
			})
		})

		Context("With two pods in the cluster", func() {
			BeforeEach(func() {
				// Create the pods for the test
				reconciler.client.Create(context.TODO(), &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s-pod", compliancescaninstance.Name, nodeinstance1.Name),
					},
				})
				reconciler.client.Create(context.TODO(), &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s-pod", compliancescaninstance.Name, nodeinstance2.Name),
					},
				})
				// Set state to RUNNING
				compliancescaninstance.Status.Phase = complianceoperatorv1alpha1.PhaseRunning
				reconciler.client.Status().Update(context.TODO(), compliancescaninstance)
			})

			It("should stay in RUNNING state", func() {
				result, err := reconciler.phaseRunningHandler(compliancescaninstance, logger)
				Expect(result).ToNot(BeNil())
				Expect(err).To(BeNil())
				Expect(compliancescaninstance.Status.Phase).To(Equal(complianceoperatorv1alpha1.PhaseRunning))
			})
		})
	})

	Context("On the DONE phase", func() {
		It("Should merely return success", func() {
			result, err := reconciler.phaseDoneHandler(compliancescaninstance, logger)
			Expect(result).ToNot(BeNil())
			Expect(err).To(BeNil())
		})
	})
})
