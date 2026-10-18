# XTM user guide

Xray Test Manager (XTM) is a desktop app for working with Xray test cases in Jira Data Center. It keeps a local copy of your project so you can browse and edit thousands of tests without waiting on Jira. Your edits are saved locally and only reach Jira when you commit them.

There is one guide for each entry in the **View** menu. Start with Getting started if you have not connected a profile yet.

| Guide | What it covers |
|---|---|
| [Getting started](00-getting-started.md) | Profiles, demo mode, syncing, committing, the More menu |
| [Browse](01-browse.md) | The test grid, the detail panel, new tests, bulk edits, importing |
| [Preconditions](02-preconditions.md) | Reusable preconditions and the tests that use them |
| [Requirements](03-requirements.md) | Requirement coverage, requirement sources, linking tests |
| [Duplicates](04-duplicates.md) | Finding and resolving tests or preconditions with the same summary |
| [Gap Analysis](05-gap-analysis.md) | Comparing your project against a CSV or XLSX test list |
| [Test Calls](06-test-calls.md) | Which tests call other tests, and broken calls |
| [Dashboard](07-dashboard.md) | Project statistics and the XLSX export |
| [Traceability](08-traceability.md) | Requirement, execution and sub-task flow charts |
| [Containers](09-containers.md) | Test Sets, Plans and Executions, run results, JUnit import, bugs |
| [Datasets](13-datasets.md) | Test defaults, Plan and Execution overrides, and their container links |
| [Components](10-components.md) | Managing the project's Jira components |
| [Coverage](11-coverage.md) | Parameter-level coverage for functional requirements |
| [Spellcheck](12-spellcheck.md) | Finding and fixing typos in test text |

## Switching views

Each view has a tab in the band under the top bar. You can also use the **View** menu in the menu bar, where the first nine views have the shortcuts Ctrl+1 to Ctrl+9:

| Shortcut | View | Shortcut | View |
|---|---|---|---|
| Ctrl+1 | Browse | Ctrl+6 | Test Calls |
| Ctrl+2 | Preconditions | Ctrl+7 | Dashboard |
| Ctrl+3 | Requirements | Ctrl+8 | Traceability |
| Ctrl+4 | Duplicates | Ctrl+9 | Containers |
| Ctrl+5 | Gap Analysis | | |

Datasets, Components, Coverage and Spellcheck are in the View menu without a shortcut. Preconditions, Requirements, Datasets and Components only appear when the profile's backend supports them. Coverage only appears after you turn it on from the **More** menu.

The screenshots in these guides come from a demo profile, so the keys start with `QA-` and the data is generated. See [Demo mode](00-getting-started.md#try-xtm-without-jira-demo-mode).
