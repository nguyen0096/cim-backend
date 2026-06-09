package main

import (
	"cim-backend/database"
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var (
	reconcileInventoryID  uint
	reconcileSubmissionID []uint
	disposeSubmissionID   []uint
	reconcileApply        bool
	reconcileOut          string
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "One-off reconcile data-fix commands (#43)",
	Long:  "Resolve pending reconcile submissions by synthesizing backdated FIFO sells.",
}

var reconcileResolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Resolve pending reconcile submissions via FIFO (preview by default)",
	Long: "Apply a fixed set of pending reconcile submissions by chronologically " +
		"chaining shrinkage drops through FIFO consume to synthesize backdated " +
		"sell transactions.\n\n" +
		"By default this runs in PREVIEW mode: it computes the full resolution " +
		"plan and persists NOTHING. Pass --apply to persist the identical plan " +
		"in a single transaction. Always review the --out plan JSON first.",
	Run: func(cmd *cobra.Command, args []string) {
		db, err := database.Initialize(config.App.Database)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to initialize database: %v\n", err)
			os.Exit(1)
		}

		// System context: synthesized sells + submission updates are stamped
		// with system@cim.local via pkg.WithUserEmail (used by Base hooks).
		ctx := pkg.WithUserEmail(context.Background(), "system@cim.local")

		inventoryRepo := repository.NewInventoryRepository(db)
		inventoryItemRepo := repository.NewInventoryItemRepository(db)
		inventorySubmissionRepo := repository.NewInventorySubmissionRepository(db)
		productRepo := repository.NewProductRepository(db)

		// fileStorageService is nil: it is never dereferenced on the
		// reconcile/consume/persist path exercised here.
		svc := services.NewInventoryService(
			inventoryRepo,
			inventoryItemRepo,
			inventorySubmissionRepo,
			productRepo,
			nil,
			db,
		)

		mode := "PREVIEW (persisting nothing)"
		if reconcileApply {
			mode = "APPLY (persisting in one transaction)"
		}
		fmt.Printf("Reconcile resolve — inventory=%d reconcile-submissions=%v dispose-submissions=%v mode=%s\n",
			reconcileInventoryID, reconcileSubmissionID, disposeSubmissionID, mode)

		// In preview, surface the inventory's pending dispose submissions so the
		// operator can choose which to fold in via --dispose-submissions.
		if !reconcileApply {
			printPendingDisposeSubmissions(db, reconcileInventoryID)
		}

		plan, err := svc.ResolvePendingSubmissions(ctx, reconcileInventoryID, reconcileSubmissionID, disposeSubmissionID, reconcileApply)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		printPlanSummary(plan)

		if reconcileOut != "" {
			data, err := json.MarshalIndent(plan, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to marshal plan: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(reconcileOut, data, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to write plan to %s: %v\n", reconcileOut, err)
				os.Exit(1)
			}
			fmt.Printf("\nWrote plan JSON to %s\n", reconcileOut)
		}

		if reconcileApply {
			fmt.Println("\nApplied. Submissions marked approved + completed.")
		} else {
			fmt.Println("\nPreview only — nothing persisted. Re-run with --apply to persist.")
		}
	},
}

// printPendingDisposeSubmissions lists the inventory's pending dispose
// submissions (id + created_at) so the operator can pick IDs for
// --dispose-submissions. Best-effort: a query error is reported but not fatal.
func printPendingDisposeSubmissions(db *gorm.DB, inventoryID uint) {
	var subs []models.InventorySubmission
	if err := db.
		Where("inventory_id = ? AND submission_type = ? AND approval_status = ?",
			inventoryID, models.InventorySubmissionTypeDispose, models.InventorySubmissionApprovalStatusPending).
		Order("created_at ASC").
		Find(&subs).Error; err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to list pending dispose submissions: %v\n", err)
		return
	}
	fmt.Printf("\nPending dispose submissions for inventory %d (%d):\n", inventoryID, len(subs))
	if len(subs) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, sub := range subs {
		fmt.Printf("  id=%d created_at=%s\n", sub.ID, sub.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Println("  Pass the chosen ids via --dispose-submissions to fold them in.")
}

func printPlanSummary(plan *dto.ResolutionPlan) {
	fmt.Println("\n=== Resolution plan summary ===")
	fmt.Printf("Inventory: %d   Submissions: %v   Applied: %t\n",
		plan.InventoryID, plan.SubmissionIDs, plan.Applied)
	fmt.Printf("Items: %d   Synthetic sells: %d   Synthetic disposals: %d   Clamped rows: %d\n",
		len(plan.Items), plan.TotalSells, plan.TotalDisposals, plan.ClampedRowCount)

	fmt.Println("\nPer-item:")
	for _, item := range plan.Items {
		fmt.Printf("  item %d (%s): start=%s totalDrop=%s disposed=%s final=%s\n",
			item.InventoryItemID, item.ProductName,
			item.StartStock.String(), item.TotalDrop.String(),
			item.TotalDisposed.String(), item.FinalStock.String())
		for _, d := range item.Drops {
			tag := ""
			if d.Clamped {
				tag = "  [CLAMPED]"
			}
			fmt.Printf("    sub %d (%s): prev=%s effPrev=%s actual=%s rawDelta=%s drop=%s%s\n",
				d.SubmissionID, d.SubmissionType, d.PrevQuantity.String(), d.EffectivePrev.String(),
				d.ActualCount.String(), d.RawDelta.String(), d.ClampedDrop.String(), tag)
		}
	}

	fmt.Printf("\nBackdated sells (%d):\n", plan.TotalSells)
	for _, s := range plan.Sells {
		fmt.Printf("  item %d qty=%s srcPurchaseTxn=%d cogs=%.2f date=%s\n",
			s.InventoryItemID, s.Quantity.String(), s.SourcePurchaseTxnID,
			s.COGSPrice, s.BackdatedDate.Format("2006-01-02 15:04:05"))
	}

	fmt.Printf("\nBackdated disposals (%d):\n", plan.TotalDisposals)
	for _, dsp := range plan.Disposals {
		fmt.Printf("  item %d qty=%s srcPurchaseTxn=%d cogs=%.2f date=%s\n",
			dsp.InventoryItemID, dsp.Quantity.String(), dsp.SourcePurchaseTxnID,
			dsp.COGSPrice, dsp.BackdatedDate.Format("2006-01-02 15:04:05"))
	}
}

func init() {
	reconcileResolveCmd.Flags().UintVar(&reconcileInventoryID, "inventory", 1, "inventory ID to resolve")
	reconcileResolveCmd.Flags().UintSliceVar(&reconcileSubmissionID, "submissions", []uint{1, 2, 4, 6}, "reconcile submission IDs to resolve (chronologically chained)")
	reconcileResolveCmd.Flags().UintSliceVar(&disposeSubmissionID, "dispose-submissions", nil, "dispose submission IDs to fold into the same chain (comma-separated; default none)")
	reconcileResolveCmd.Flags().BoolVar(&reconcileApply, "apply", false, "persist the plan (default: preview only)")
	reconcileResolveCmd.Flags().StringVar(&reconcileOut, "out", "", "write the resolution plan as JSON to this path")

	reconcileCmd.AddCommand(reconcileResolveCmd)
	rootCmd.AddCommand(reconcileCmd)
}
