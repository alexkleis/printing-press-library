// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.
// Hand-edit (NOT generated): models the search-recording resource added
// to spec.yaml on 2026-05-25. Mirrors promoted_search-name.go for the
// recordingsearch endpoint discovered in the portal's SPA bundle.
// See docs/superpowers/specs/2026-05-25-book-page-back-ref-design.md in
// emma-project-v2 for the design + portal API contract.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/cliutil"
)

func newSearchRecordingPromotedCmd(flags *rootFlags) *cobra.Command {
	var bodyBookNumber string
	var bodyPageNumber string
	var bodyBookType string
	var bodySearchType string

	cmd := &cobra.Command{
		Use:         "search-recording",
		Short:       "Submit a Recording Book/Page search. Returns an encrypted qs token used to fetch the full result set.",
		Long:        "Submit a Recording Book/Page search (a.k.a. Document Reference search). For a given underlying mortgage's book/page, the clerk returns every record indexed against it — typically SAT/REL/AMO/SMO/CLP. Bypasses name-search ambiguity because the clerk's own index is the truth source.",
		Example:     "  miami-dade-clerk-pp-cli search-recording --book-number 25647 --page-number 916 --book-type O --search-type \"Recording Book/Page\"",
		Annotations: map[string]string{"pp:endpoint": "search-recording.submit", "pp:method": "POST", "pp:path": "/api/home/recordingsearch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("book-number") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "book-number")
			}
			if !cmd.Flags().Changed("page-number") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "page-number")
			}
			if !cmd.Flags().Changed("book-type") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "book-type")
			}
			if !cmd.Flags().Changed("search-type") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "search-type")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/api/home/recordingsearch"
			params := map[string]string{}
			body := map[string]any{}
			if bodyBookNumber != "" {
				body["bookNumber"] = bodyBookNumber
			}
			if bodyPageNumber != "" {
				body["pageNumber"] = bodyPageNumber
			}
			if bodyBookType != "" {
				body["bookType"] = bodyBookType
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
	cmd.Flags().StringVar(&bodyBookNumber, "book-number", "", "Recording book number (digits only)")
	cmd.Flags().StringVar(&bodyPageNumber, "page-number", "", "Recording page number (digits only)")
	cmd.Flags().StringVar(&bodyBookType, "book-type", "", "Book type code — typically 'O' for Official Records")
	cmd.Flags().StringVar(&bodySearchType, "search-type", "", "Always 'Recording Book/Page'")

	return cmd
}
