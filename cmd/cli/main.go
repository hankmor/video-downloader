package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "vdd-cli",
		Usage: "VDD CLI (open-source edition)",
		Commands: []*cli.Command{
			{
				Name:  "debug",
				Usage: "Local debug tools",
				Subcommands: []*cli.Command{
					{Name: "expire", Usage: "Set first-run date to 10 days ago", Action: actionDebugExpire},
					{Name: "reset", Usage: "Reset first-run status", Action: actionDebugReset},
					{Name: "info", Usage: "Show local status", Action: actionDebugInfo},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func actionDebugExpire(c *cli.Context) error {
	ops := DebugOps{}
	expiredDate := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	if err := ops.SetTrialDate(expiredDate); err != nil {
		return err
	}
	fmt.Printf("First-run date set to %s\n", expiredDate)
	return nil
}

func actionDebugReset(c *cli.Context) error {
	ops := DebugOps{}
	if err := ops.ResetTrial(); err != nil {
		return err
	}
	fmt.Println("First-run status reset successful.")
	return nil
}

func actionDebugInfo(c *cli.Context) error {
	ops := DebugOps{}
	info := ops.GetInfo()

	fmt.Println("=== Local VDD Status ===")
	fmt.Printf("First Run Date: %s\n", info["first_run_date"])
	fmt.Printf("Days Left:      %s\n", info["days_left"])
	fmt.Printf("Mode:           %s\n", info["mode"])
	return nil
}
