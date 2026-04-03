package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/NoTIPswe/notip-simulator-cli/internal/client"
	"github.com/spf13/cobra"
)

var anomaliesCmd = &cobra.Command{
	Use:   "anomalies",
	Short: "Trigger anomaly scenarios on gateways and sensors",
}

var exitProcess = os.Exit

func mustMarkRequired(cmd *cobra.Command, flagName string) {
	if err := cmd.MarkFlagRequired(flagName); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exitProcess(1)
	}
}

// ── disconnect ────────────────────────────────────────────────────────────────

var anomaliesDisconnectCmd = &cobra.Command{
	Use:   "disconnect <gateway-uuid>",
	Short: "Simulate a gateway disconnect for a given duration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		duration, _ := cmd.Flags().GetInt("duration")

		spinner := startSpinner(
			fmt.Sprintf("Triggering disconnect anomaly on gateway %s (%ds)...", args[0], duration),
		)
		if err := client.New(simulatorURL).Disconnect(args[0], duration); err != nil {
			spinner.Fail("Failed to trigger disconnect")
			return err
		}
		spinner.Success(fmt.Sprintf("Disconnect anomaly triggered on gateway %s", args[0]))
		return nil
	},
}

// ── network-degradation ───────────────────────────────────────────────────────

var anomaliesNetworkDegradationCmd = &cobra.Command{
	Use:   "network-degradation <gateway-uuid>",
	Short: "Simulate network degradation on a gateway (packet loss)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		duration, _ := cmd.Flags().GetInt("duration")
		loss, _ := cmd.Flags().GetFloat64("packet-loss")

		spinner := startSpinner(
			fmt.Sprintf("Triggering network-degradation on gateway %s (%ds, %.0f%% loss)...",
				args[0], duration, loss*100),
		)
		if err := client.New(simulatorURL).InjectNetworkDegradation(args[0], duration, loss); err != nil {
			spinner.Fail("Failed to trigger network degradation")
			return err
		}
		spinner.Success(fmt.Sprintf("Network degradation triggered on gateway %s", args[0]))
		return nil
	},
}

// ── outlier ───────────────────────────────────────────────────────────────────

var anomaliesOutlierCmd = &cobra.Command{
	Use:   "outlier <sensor-int-id>",
	Short: "Inject an outlier reading into a sensor (uses the numeric sensor ID)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sensorID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("sensor-id must be a numeric ID: %w", err)
		}

		var valuePtr *float64
		if cmd.Flags().Changed("value") {
			v, _ := cmd.Flags().GetFloat64("value")
			valuePtr = &v
		}

		spinner := startSpinner(
			fmt.Sprintf("Injecting outlier into sensor %d...", sensorID),
		)
		if err := client.New(simulatorURL).InjectOutlier(sensorID, valuePtr); err != nil {
			spinner.Fail("Failed to inject outlier")
			return err
		}
		spinner.Success(fmt.Sprintf("Outlier injected into sensor %d", sensorID))
		return nil
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	rootCmd.AddCommand(anomaliesCmd)
	anomaliesCmd.AddCommand(
		anomaliesDisconnectCmd,
		anomaliesNetworkDegradationCmd,
		anomaliesOutlierCmd,
	)

	// disconnect flags
	anomaliesDisconnectCmd.Flags().Int("duration", 0, "Disconnect duration in seconds (required, must be > 0)")
	mustMarkRequired(anomaliesDisconnectCmd, "duration")

	// network-degradation flags
	anomaliesNetworkDegradationCmd.Flags().Int("duration", 0, "Duration in seconds (required)")
	anomaliesNetworkDegradationCmd.Flags().Float64("packet-loss", 0, "Packet loss fraction 0–1 (e.g. 0.3 = 30%); omit to use backend default of 0.3")
	mustMarkRequired(anomaliesNetworkDegradationCmd, "duration")

	// outlier flags
	anomaliesOutlierCmd.Flags().Float64("value", 0, "Outlier value to inject; omit to let the backend decide")
}
