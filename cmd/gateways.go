package cmd

import (
	"fmt"
	"strconv"

	"github.com/NoTIPswe/notip-simulator-cli/internal/client"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

const (
	flagFactoryID  = "factory-id"
	flagFactoryKey = "factory-key"
)

var gatewaysCmd = &cobra.Command{
	Use:   "gateways",
	Short: "Manage simulator gateways",
}

// ── list ──────────────────────────────────────────────────────────────────────

var gatewaysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all gateways and their current status",
	RunE: func(cmd *cobra.Command, args []string) error {
		spinner := startSpinner("Fetching gateways...")

		c := client.New(simulatorURL).WithContext(cmd.Context())
		gateways, err := c.ListGateways()
		if err != nil {
			spinner.Fail("Failed to fetch gateways")
			return err
		}
		spinner.Success("Gateways retrieved")

		if len(gateways) == 0 {
			pterm.Info.Println("No gateways found.")
			return nil
		}

		tableData := pterm.TableData{
			{"ID", "UUID", "Status", "Model", "Freq (ms)", "Tenant"},
		}
		for _, gw := range gateways {
			tableData = append(tableData, []string{
				strconv.FormatInt(gw.ID, 10),
				gw.ManagementGatewayID,
				statusStyle(gw.Status),
				gw.Model,
				strconv.Itoa(gw.SendFrequencyMs),
				gw.TenantID,
			})
		}
		return pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	},
}

// ── get ───────────────────────────────────────────────────────────────────────

var gatewaysGetCmd = &cobra.Command{
	Use:   "get <uuid>",
	Short: "Show details for a single gateway",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spinner := startSpinner("Fetching gateway " + args[0] + "...")

		c := client.New(simulatorURL).WithContext(cmd.Context())
		gw, err := c.GetGateway(args[0])
		if err != nil {
			spinner.Fail("Failed to fetch gateway")
			return err
		}
		spinner.Success("Gateway retrieved")

		return pterm.DefaultTable.WithData(pterm.TableData{
			{"Field", "Value"},
			{"ID", strconv.FormatInt(gw.ID, 10)},
			{"UUID", gw.ManagementGatewayID},
			{"Factory ID", gw.FactoryID},
			{"Model", gw.Model},
			{"Firmware", gw.FirmwareVersion},
			{"Status", statusStyle(gw.Status)},
			{"Provisioned", strconv.FormatBool(gw.Provisioned)},
			{"Send Freq (ms)", strconv.Itoa(gw.SendFrequencyMs)},
			{"Tenant", gw.TenantID},
			{"Created At", gw.CreatedAt},
		}).Render()
	},
}

// ── create (single) ───────────────────────────────────────────────────────────

var gatewaysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a single gateway (POST /sim/gateways)",
	RunE: func(cmd *cobra.Command, args []string) error {
		req := client.CreateGatewayRequest{}
		req.FactoryID, _ = cmd.Flags().GetString(flagFactoryID)
		req.FactoryKey, _ = cmd.Flags().GetString(flagFactoryKey)
		req.SerialNumber, _ = cmd.Flags().GetString("serial")
		req.Model, _ = cmd.Flags().GetString("model")
		req.FirmwareVersion, _ = cmd.Flags().GetString("firmware")
		req.SendFrequencyMs, _ = cmd.Flags().GetInt("freq")

		spinner := startSpinner("Creating gateway...")
		c := client.New(simulatorURL).WithContext(cmd.Context())
		gw, err := c.CreateGateway(req)
		if err != nil {
			spinner.Fail("Failed to create gateway")
			return err
		}
		spinner.Success("Gateway created")
		printGatewayTable([]client.Gateway{*gw})
		return nil
	},
}

// ── bulk ──────────────────────────────────────────────────────────────────────

var gatewaysBulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Create multiple gateways at once (POST /sim/gateways/bulk)",
	RunE: func(cmd *cobra.Command, args []string) error {
		req := client.BulkCreateGatewaysRequest{}
		req.Count, _ = cmd.Flags().GetInt("count")
		req.FactoryID, _ = cmd.Flags().GetString(flagFactoryID)
		req.FactoryKey, _ = cmd.Flags().GetString(flagFactoryKey)
		req.Model, _ = cmd.Flags().GetString("model")
		req.FirmwareVersion, _ = cmd.Flags().GetString("firmware")
		req.SendFrequencyMs, _ = cmd.Flags().GetInt("freq")

		spinner := startSpinner(
			fmt.Sprintf("Creating %d gateway(s)...", req.Count),
		)
		c := client.New(simulatorURL).WithContext(cmd.Context())
		result, err := c.BulkCreateGateways(req)
		if err != nil {
			spinner.Fail("Failed to create gateways")
			return err
		}

		created := len(result.Gateways)
		failed := 0
		for _, e := range result.Errors {
			if e != "" {
				failed++
			}
		}
		if failed > 0 {
			spinner.Warning(fmt.Sprintf("%d created, %d failed (207 Partial)", created, failed))
			for i, e := range result.Errors {
				if e != "" {
					pterm.Error.Printf("  [%d] %s\n", i, e)
				}
			}
		} else {
			spinner.Success(fmt.Sprintf("%d gateway(s) created", created))
		}

		printGatewayTable(result.Gateways)
		return nil
	},
}

// ── start ─────────────────────────────────────────────────────────────────────

var gatewaysStartCmd = &cobra.Command{
	Use:   "start <uuid>",
	Short: "Start telemetry emission for a gateway",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spinner := startSpinner("Starting gateway " + args[0] + "...")
		if err := client.New(simulatorURL).WithContext(cmd.Context()).StartGateway(args[0]); err != nil {
			spinner.Fail("Failed to start gateway")
			return err
		}
		spinner.Success("Gateway started")
		return nil
	},
}

// ── stop ──────────────────────────────────────────────────────────────────────

var gatewaysStopCmd = &cobra.Command{
	Use:   "stop <uuid>",
	Short: "Stop telemetry emission for a gateway",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spinner := startSpinner("Stopping gateway " + args[0] + "...")
		if err := client.New(simulatorURL).WithContext(cmd.Context()).StopGateway(args[0]); err != nil {
			spinner.Fail("Failed to stop gateway")
			return err
		}
		spinner.Success("Gateway stopped")
		return nil
	},
}

// ── delete ────────────────────────────────────────────────────────────────────

var gatewaysDeleteCmd = &cobra.Command{
	Use:   "delete <uuid>",
	Short: "Delete a gateway by UUID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spinner := startSpinner("Deleting gateway " + args[0] + "...")
		if err := client.New(simulatorURL).WithContext(cmd.Context()).DeleteGateway(args[0]); err != nil {
			spinner.Fail("Failed to delete gateway")
			return err
		}
		spinner.Success("Gateway deleted")
		return nil
	},
}

// ── helpers ───────────────────────────────────────────────────────────────────

func statusStyle(status string) string {
	if !pterm.RawOutput {
		switch status {
		case "online", "connected":
			return pterm.Green(status)
		case "offline", "disconnected":
			return pterm.Red(status)
		}
	}
	return status
}

func printGatewayTable(gateways []client.Gateway) {
	if len(gateways) == 0 {
		return
	}
	tableData := pterm.TableData{{"ID", "UUID", "Status", "Model", "Freq (ms)"}}
	for _, gw := range gateways {
		tableData = append(tableData, []string{
			strconv.FormatInt(gw.ID, 10),
			gw.ManagementGatewayID,
			statusStyle(gw.Status),
			gw.Model,
			strconv.Itoa(gw.SendFrequencyMs),
		})
	}
	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render() //nolint:errcheck
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	rootCmd.AddCommand(gatewaysCmd)
	gatewaysCmd.AddCommand(
		gatewaysListCmd,
		gatewaysGetCmd,
		gatewaysCreateCmd,
		gatewaysBulkCmd,
		gatewaysStartCmd,
		gatewaysStopCmd,
		gatewaysDeleteCmd,
	)

	// create flags
	gatewaysCreateCmd.Flags().String(flagFactoryID, "", "Factory ID (required)")
	gatewaysCreateCmd.Flags().String(flagFactoryKey, "", "Factory key (required)")
	gatewaysCreateCmd.Flags().String("serial", "", "Serial number (required)")
	gatewaysCreateCmd.Flags().String("model", "", "Gateway model (required)")
	gatewaysCreateCmd.Flags().String("firmware", "", "Firmware version (required)")
	gatewaysCreateCmd.Flags().Int("freq", 1000, "Send frequency in milliseconds (required)")
	for _, f := range []string{flagFactoryID, flagFactoryKey, "serial", "model", "firmware", "freq"} {
		mustMarkRequired(gatewaysCreateCmd, f)
	}

	// bulk flags
	gatewaysBulkCmd.Flags().Int("count", 1, "Number of gateways to create (required)")
	gatewaysBulkCmd.Flags().String(flagFactoryID, "", "Factory ID (required)")
	gatewaysBulkCmd.Flags().String(flagFactoryKey, "", "Factory key (required)")
	gatewaysBulkCmd.Flags().String("model", "", "Gateway model (required)")
	gatewaysBulkCmd.Flags().String("firmware", "", "Firmware version (required)")
	gatewaysBulkCmd.Flags().Int("freq", 1000, "Send frequency in milliseconds (required)")
	for _, f := range []string{"count", flagFactoryID, flagFactoryKey, "model", "firmware", "freq"} {
		mustMarkRequired(gatewaysBulkCmd, f)
	}
}
