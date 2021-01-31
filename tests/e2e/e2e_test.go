package e2e_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puppetlabs/leg/k8sutil/pkg/test/endtoend"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var schemes = runtime.NewSchemeBuilder(
	scheme.AddToScheme,
	pvpoolv1alpha1.AddToScheme,
)

func init() {
	log.SetLogger(klogr.NewWithOptions(klogr.WithFormat(klogr.FormatKlog)))
}

type EnvironmentInTest struct {
	*endtoend.Environment
	Labels      map[string]string
	PoolHelpers *PoolHelpers
	t           *testing.T
	nf          endtoend.NamespaceFactory
}

func (eit *EnvironmentInTest) WithNamespace(ctx context.Context, fn func(ns *corev1.Namespace)) {
	require.NoError(eit.t, endtoend.WithNamespace(ctx, eit.Environment, eit.nf, fn))
}

func WithEnvironmentInTest(t *testing.T, fn func(eit *EnvironmentInTest)) {
	viper.SetEnvPrefix("pvpool_test_e2e")
	viper.AutomaticEnv()

	kubeconfigs := strings.TrimSpace(viper.GetString("kubeconfig"))
	if testing.Short() {
		t.Skip("not running end-to-end tests with -short")
	} else if kubeconfigs == "" {
		t.Skip("not running end-to-end tests without one or more Kubeconfigs specified by PVPOOL_TEST_E2E_KUBECONFIG")
	}

	s := runtime.NewScheme()
	require.NoError(t, schemes.AddToScheme(s))

	opts := []endtoend.EnvironmentOption{
		endtoend.EnvironmentWithClientScheme(s),
		endtoend.EnvironmentWithClientKubeconfigs(filepath.SplitList(kubeconfigs)),
		endtoend.EnvironmentWithClientContext(viper.GetString("context")),
	}

	require.NoError(t, endtoend.WithEnvironment(opts, func(e *endtoend.Environment) {
		ls := map[string]string{
			"e2e.tests.pvpool.puppet.com/harness":   "end-to-end",
			"e2e.tests.pvpool.puppet.com/test.hash": testHash(t),
		}

		eit := &EnvironmentInTest{
			Environment: e,
			Labels:      ls,
			t:           t,
			nf:          endtoend.NewTestNamespaceFactory(t, endtoend.NamespaceWithLabels(ls)),
		}
		eit.PoolHelpers = &PoolHelpers{
			eit: eit,
		}
		fn(eit)
	}))
}

func testHash(t *testing.T) string {
	h := sha256.Sum256([]byte(t.Name()))
	return hex.EncodeToString(h[:])[:63]
}
