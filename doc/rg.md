                       ┌────────────────────────────────────────────────────────┐
                       │                   Search / Source                      │
                       │  • :rg <query> / K (CmdRgUnderCursor)                  │
                       │  • Live RG Picker (Ctrl-r in CmdSearchProject)         │
                       │  • Explicit LSP (:diags / CmdLspDiagnosticsToQuickfix) │
                       └───────────┬────────────────────────────────┬───────────┘
                                   │                                │
                                   ▼                                ▼
                     ┌───────────────────────────┐    ┌───────────────────────────┐
                     │   rg_search.json (file)   │    │   quickfix.json (file)    │
                     │   • Persists raw RgResult │    │   • Persists QuickfixEntry│
                     │   • 1-indexed Line        │    │   • 0-indexed Line        │
                     └─────────────┬─────────────┘    └─────────────┬─────────────┘
                                   │                                │
                                   ▼                                ▼
                     ┌───────────────────────────┐    ┌───────────────────────────┐
                     │     [rg] Grouped Buffer   │    │  [quickfix] Buffer/Popup  │
                     │   • F11 (CmdOpenSaved)    │    │   • :copen                │
                     │   • File-grouped layout   │    │   • Respects quickfix_view│
                     │   • Jump with l / L       │    │     ("split" or "popup")  │
                     └─────────────┬─────────────┘    └─────────────┬─────────────┘
                                   │                                │
                                   └────────────────┬───────────────┘
                                                    │
                                                    ▼
                                      ┌───────────────────────────┐
                                      │        :cn / :cp          │
                                      │ Displays [x/N matches]    │
                                      └───────────────────────────┘
                                      
2. Verification of Key Workflows
A. :rg <query> & K (under cursor)

    Execution: Runs rg --json -S <word> in the project root via rgSearchLocations.
    Persistence & Sync:
        Saves to ~/.config/wig/rg_search.json via rgcollect.SaveResults(locations).
        Parses and syncs to ~/.config/wig/quickfix.json via SetQuickfixLocations(locations).
    Display: Opens the full-screen grouped [rg] buffer (rgcollect.InitGrouped).
    Navigation: Setting wig.SetVisitSource(buf) enables :cn and :cp immediately via visitLineGrouped, echoing [x/N matches] <text>.

B. Live Project Search + <Ctrl-r> (CmdSearchProject)
    Execution: User types a live query in the picker and presses Ctrl-r.
    Persistence & Sync:
        Saves filtered matches to rg_search.json (rgcollect.SaveResults(locs)).
        Saves and syncs to quickfix.json (OpenLocationsInQuickfix(c, locs)).
    Display: Opens the quickfix view configured in user settings (quickfix_view = "popup" or "split").
    Navigation: Sets wig.SetVisitSource(qfBuf) so :cn / :cp works via visitQuickfixLine, echoing [x/N matches] <text>.

C. Reopening Grouped Search (F11 / CmdOpenSavedSearch)
    Execution: Calls rgcollect.LoadResults() from ~/.config/wig/rg_search.json.
    Sync: Calls SetQuickfixLocations(locations) so quickfix.json matches the reopened search.
    Display: Restores the grouped [rg] buffer.

D. Quickfix Viewer (:copen) & LSP Diagnostics (:diags / :diagnostics)
    :copen (Pure Viewer): Opens the existing in-memory quickfix list, or loads from quickfix.json if restarted. Does not overwrite your search results with local file LSP diagnostics.
    :diags / :diagnostics (Explicit): Specifically dumps current buffer LSP diagnostics into quickfix.json only when explicitly invoked.                                      
