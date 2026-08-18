// dns-doctor — kubectl plugin that narrows down Kubernetes DNS problems.
package main

import (
	"fmt"
	"os"

	"github.com/phoenix-platform/dns-doctor/internal/output"
	"github.com/phoenix-platform/dns-doctor/internal/runner"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

const version = "0.1.0"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		namespace  string
		pod        string
		outputFmt  string
		probeMode  bool
		kubeconfig string
		kubeCtx    string
	)

	cmd := &cobra.Command{
		Use:   "dns-doctor",
		Short: "One command to narrow down Kubernetes DNS problems.",
		Long: `dns-doctor inspects your cluster's DNS configuration and reports
potential issues with CoreDNS, NetworkPolicies, and workload DNS settings.

All data stays on your machine — nothing is sent externally.`,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if probeMode {
				fmt.Fprintln(os.Stdout, "[INFO] --probe is not yet implemented.")
				fmt.Fprintln(os.Stdout)
				fmt.Fprintln(os.Stdout, "When implemented, --probe will:")
				fmt.Fprintln(os.Stdout, "  1. Create a temporary Pod (image: busybox:1.35, namespace: kube-system or the target namespace)")
				fmt.Fprintln(os.Stdout, "     with a TTL of 60 seconds and a unique name prefixed 'dns-doctor-probe-'.")
				fmt.Fprintln(os.Stdout, "  2. Execute 'nslookup kubernetes.default.svc.cluster.local' inside that pod.")
				fmt.Fprintln(os.Stdout, "  3. Delete the pod immediately after receiving output (or on error/timeout).")
				fmt.Fprintln(os.Stdout, "  4. Report the DNS resolution result.")
				fmt.Fprintln(os.Stdout)
				fmt.Fprintln(os.Stdout, "The probe pod will NEVER be left running. It will be deleted on completion or SIGINT.")
				fmt.Fprintln(os.Stdout, "This feature is NOT the default mode. Re-run without --probe for passive analysis.")
				return nil
			}

			// Build kubeconfig loading rules.
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			if kubeconfig != "" {
				loadingRules.ExplicitPath = kubeconfig
			}
			configOverrides := &clientcmd.ConfigOverrides{}
			if kubeCtx != "" {
				configOverrides.CurrentContext = kubeCtx
			}

			kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
			restCfg, err := kubeConfig.ClientConfig()
			if err != nil {
				return fmt.Errorf("could not build kubeconfig: %w", err)
			}

			client, err := kubernetes.NewForConfig(restCfg)
			if err != nil {
				return fmt.Errorf("could not create Kubernetes client: %w", err)
			}

			// Load raw config for context/cluster name extraction (check 2).
			var rawCfg *api.Config
			raw, err := kubeConfig.RawConfig()
			if err == nil {
				rawCfg = &raw
			}

			opts := runner.Options{
				Namespace: namespace,
				Pod:       pod,
				ProbeMode: probeMode,
			}

			ctx := cmd.Context()
			results, cluster, contextName := runner.Run(ctx, client, rawCfg, opts)

			ns := namespace
			if ns == "" {
				ns = "default"
			}

			if outputFmt == "json" {
				return output.WriteJSON(os.Stdout, version, cluster, contextName, ns, results)
			}

			p := output.NewPrinter()
			p.PrintAll(results)
			p.PrintSummaryLine(results)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&namespace, "namespace", "n", "", "Namespace to inspect (default: default)")
	flags.StringVar(&pod, "pod", "", "Pod name to inspect DNS config for (optional)")
	flags.StringVarP(&outputFmt, "output", "o", "human", "Output format: human | json")
	flags.BoolVar(&probeMode, "probe", false, "Run an active DNS probe from inside the cluster (NOT YET IMPLEMENTED)")
	flags.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	flags.StringVar(&kubeCtx, "context", "", "kubeconfig context to use (overrides current-context)")

	return cmd
}
