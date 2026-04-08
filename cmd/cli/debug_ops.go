package main

import (
	"fmt"
	"time"

	"github.com/hankmor/vdd/core/auth"
	"github.com/hankmor/vdd/core/config"
	"github.com/hankmor/vdd/core/db"
)

// DebugOps provides debug operations for local testing
type DebugOps struct{}

// helper to ensure DB is initialized
func ensureDB() error {
	if db.DB == nil {
		// Debug ops only need ConfigModel usually
		if err := db.Init(&config.ConfigModel{}); err != nil {
			return err
		}
	}
	return nil
}

// SetTrialDate sets first_run_date to a specific date
// format: YYYY-MM-DD
func (d *DebugOps) SetTrialDate(date string) error {
	if err := ensureDB(); err != nil {
		return err
	}
	// Verify format
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}

	config.DAO.Set(tx, "first_run_date", date)

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// ResetTrial clears first-run related data
func (d *DebugOps) ResetTrial() error {
	if err := ensureDB(); err != nil {
		return err
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}

	// Using empty string to clear
	config.DAO.Set(tx, "first_run_date", "")

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// GetInfo returns current status
func (d *DebugOps) GetInfo() map[string]string {
	ensureDB()    // Ensure DB loaded
	config.Load() // Ensure loaded
	info := make(map[string]string)

	info["first_run_date"] = config.Get().FirstRunDate
	info["days_left"] = fmt.Sprintf("%d", auth.GetAutherization().UserTrialDaysLeft())
	info["mode"] = "open-source"

	return info
}
