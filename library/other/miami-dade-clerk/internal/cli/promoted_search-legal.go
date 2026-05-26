// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.
// Hand-edit (NOT generated): models the search-legal resource added to
// spec.yaml on 2026-05-27. Mirrors promoted_search-recording.go for the
// legaldescriptionsearch endpoint discovered in the portal's SPA bundle.
//
// Why this exists: the clerk's "Legal Description" search returns the
// canonical conveyance chain (DEEDS + MORTGAGES + LIS + QCDs) indexed
// against a specific plat (book, page, block, lot). This is Phase-4
// "Layer 0" in the title-search architecture — the source of truth for
// what records belong to a lot's chain-of-title. The Go CLI exposes
// search-name (party-indexed), search-property (address-indexed), and
// search-recording (book/page back-ref); search-legal completes the set.
// See docs/superpowers/handoffs/2026-05-26-title-search-phase-4-demo-complete.md
// in emma-project-v2 for the design + portal API contract.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/cliutil"
)

func newSearchLegalPromotedCmd(flags *rootFlags) *cobra.Command {
	var bodyPlatBook string
	var bodyPlatPage string
	var bodyBlockNo string
	var bodyLotNo string
	var bodySubdivisionName string
	var bodySearchType string

	cmd := &cobra.Command{
		Use:         "search-legal",
		Short:       "Submit a Legal Description search. Returns an encrypted qs token used to fetch the full result set.",
		Long:        "Submit a Legal Description search (a.k.a. plat-search). For a given plat book + page + block + lot, the clerk returns every record indexed against that lot's legal description — DEEDS + MORTGAGES + LIS + QCDs. This is the canonical conveyance chain. Complementary to search-name (parties) and search-property (addresses); together they form the title-search Layer 0 + Layer 5 pair.",
		Example:     "  miami-dade-clerk-pp-cli search-legal --plat-book 71 --plat-page 560 --block-no 18 --lot-no 8 --search-type \"Legal Description\"",
		Annotations: map[string]string{"pp:endpoint": "search-legal.submit", "pp:method": "POST", "pp:path": "/api/home/legaldescriptionsearch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// plat-book + plat-page are required to identify a plat. The
			// block + lot narrow within that plat. searchType is the
			// portal's enum discriminator.
			if !cmd.Flags().Changed("plat-book") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "plat-book")
			}
			if !cmd.Flags().Changed("plat-page") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "plat-page")
			}
			if !cmd.Flags().Changed("search-type") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "search-type")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/api/home/legaldescriptionsearch"
			params := map[string]string{}
			body := map[string]any{}
			if bodyPlatBook != "" {
				body["platBook"] = bodyPlatBook
			}
			if bodyPlatPage != "" {
				body["platPage"] = bodyPlatPage
			}
			if bodyBlockNo != "" {
				body["blockNo"] = bodyBlockNo
			}
			if bodyLotNo != "" {
				body["lotNo"] = bodyLotNo
			}
			if bodySubdivisionName != "" {
				body["subdivisionName"] = bodySubdivisionName
			}
			if bodySearchType != "" {
				body["searchType"] = bodySearchType
			}
			// Use the reCAPTCHA-aware variant: mints a fresh x-recaptcha-token
			// via headless Chrome (chromedp) before each call. Falls back to
			// the unprotected path under --dry-run / PRINTING_PRESS_VERIFY=1.
			var data []byte
			if flags.dryRun || cliutil.IsVerifyEnv() {
				data, _, err = c.PostWithParams(path, params, body)
			} else {
				queryParams := map[string]string{}
				for k, v := range body {
					if s, ok := v.(string); ok {
						queryParams[k] = s
					}
				}
				var rawData json.RawMessage
				rawData, _, err = c.PostSearchWithRecaptcha(cmd.Context(), path, queryParams)
				data = []byte(rawData)
			}

			prov := attachFreshness(DataProvenance{Source: "live"}, flags)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				if json.Unmarshal(data, &countItems) != nil {
					countItems = []json.RawMessage{data}
				}
				printProvenance(cmd, len(countItems), prov)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&bodyPlatBook, "plat-book", "", "Plat book number (digits only)")
	cmd.Flags().StringVar(&bodyPlatPage, "plat-page", "", "Plat page number — note Miami-Dade indexes this as ×10 of the human-readable form (PB 71 Page 56 → query platPage=560)")
	cmd.Flags().StringVar(&bodyBlockNo, "block-no", "", "Block number/letter within the plat (optional — broadens to whole plat when omitted)")
	cmd.Flags().StringVar(&bodyLotNo, "lot-no", "", "Lot number within the block (optional — broadens to whole block when omitted; filter to lot client-side via legaL_DESCRIPTION ^LOT N regex)")
	cmd.Flags().StringVar(&bodySubdivisionName, "subdivision-name", "", "Subdivision name (optional — accepted but rarely needed when plat-book + plat-page are supplied)")
	cmd.Flags().StringVar(&bodySearchType, "search-type", "", "Always 'Legal Description'")

	return cmd
}
