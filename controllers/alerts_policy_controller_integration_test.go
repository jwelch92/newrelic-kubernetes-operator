// +build integration

package controllers

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/newrelic-client-go/pkg/alerts"

	nralertsv1 "github.com/newrelic/newrelic-kubernetes-operator/api/v1"
	nrv1 "github.com/newrelic/newrelic-kubernetes-operator/api/v1"
	"github.com/newrelic/newrelic-kubernetes-operator/interfaces"
)

func alertsPolicyTestSetup(t *testing.T, policy *nrv1.AlertsPolicy) client.Client {
	ctx := context.Background()
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "configs", "crd", "bases")},
	}

	// var err error
	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	err = nralertsv1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	// +kubebuilder:scaffold:scheme

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)

	err = k8sClient.Create(ctx, policy)
	require.NoError(t, err)

	return k8sClient
}

func TestIntegrationAlertsPolicyController(t *testing.T) {
	t.Parallel()

	accountID, err := strconv.Atoi(os.Getenv("NEW_RELIC_ACCOUNT_ID"))
	assert.NoError(t, err)

	conditionSpec := &nrv1.AlertsNrqlConditionSpec{
		Terms: []nrv1.AlertsConditionTerm{
			{
				Operator:          "above",
				Priority:          "critical",
				Threshold:         "5",
				ThresholdDuration: 30,
				TimeFunction:      "all",
			},
		},
		Nrql: alerts.NrqlConditionQuery{
			Query:            "SELECT 1 FROM MyEvents",
			EvaluationOffset: 5,
		},
		Type:               "NRQL",
		Name:               "NRQL Condition",
		RunbookURL:         "http://test.com/runbook",
		ValueFunction:      &alerts.NrqlConditionValueFunctions.SingleValue,
		ViolationTimeLimit: alerts.NrqlConditionViolationTimeLimits.OneHour,
		ExpectedGroups:     2,
		IgnoreOverlap:      true,
		Enabled:            true,
		// ExistingPolicyID:   integrationPolicy.ID,
		// APIKey:             integrationAlertsConfig.PersonalAPIKey,
		// AccountID:          accountID,

	}

	policy := &nrv1.AlertsPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: nrv1.AlertsPolicySpec{
			Name:               "test policy",
			APIKey:             os.Getenv("NEW_RELIC_API_KEY"),
			IncidentPreference: "PER_POLICY",
			Region:             "us",
			Conditions: []nrv1.AlertsPolicyCondition{
				{
					Spec: *conditionSpec,
				},
			},
			AccountID: accountID,
		},
		Status: nrv1.AlertsPolicyStatus{
			AppliedSpec: &nrv1.AlertsPolicySpec{},
			PolicyID:    0,
		},
	}

	// Must come before calling reconciler.Reconcile()
	k8sClient := alertsPolicyTestSetup(t, policy)

	namespacedName := types.NamespacedName{
		Namespace: "default",
		Name:      "test-policy",
	}

	request := ctrl.Request{
		NamespacedName: namespacedName,
	}

	reconciler := &AlertsPolicyReconciler{
		Client:          k8sClient,
		Log:             logf.Log,
		AlertClientFunc: interfaces.InitializeAlertsClient,
	}

	// call reconcile
	result, err := reconciler.Reconcile(request)
	assert.NoError(t, err)
	t.Logf("\n\n RESULT: %+v \n\n", result)

	// Deferred teardown
	// defer func() {
	// 	_, err := client.DeletePolicy(policy.ID)

	// 	if err != nil {
	// 		t.Logf("error cleaning up alert policy %d (%s): %s", policy.ID, policy.Name, err)
	// 	}
	// }()
}