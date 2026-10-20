# Browse

Browse is where you find, read and edit test cases. The left sidebar groups the tests, the grid lists them, and the detail panel on the right shows one test at a time. Open it with the **Browse** tab or Ctrl+1.

![The Browse view](images/browse-01-grid.png)

## Find tests

### Group the sidebar

The list at the top of the sidebar picks how tests are grouped:

- **Group by: Folder** shows the Xray Test Repository folders as a tree. This option appears when the backend supports folders.
- **Group by: Test Set** and **Group by: Test Plan** list containers. Click one to see its tests.
- **Group by: Component** lists the project's components.

Click a folder to show only its tests. The number next to each folder is its test count, subfolders included. **All tests** at the top clears the selection.

![The Checkout folder selected](images/browse-02-folder.png)

![Grouping by Test Plan](images/browse-04-group-by-plan.png)

In folder mode, hover over a folder to see its buttons: new top-level folder, new test in this folder, new subfolder, rename and delete. **+ New folder** at the bottom of the sidebar creates a top-level folder. Folder changes are pending changes like any other edit.

### Search and filter the grid

The row above the grid filters what it shows:

- **Search key, summary, description…** matches as you type.
- **Status…** filters by workflow status.
- **Any exec type** limits the grid to Manual, Cucumber, Generic or Automated tests.

![Searching for "guest" in the Checkout folder](images/browse-03-search.png)

To keep a combination of filters, click **Save view** and give it a name. **Saved views…** brings it back, and **✕** next to an active view deletes it.

Click a column header (Key, Summary, Status, Updated) to sort. The pager at the bottom moves between pages and sets the number of rows per page.

### Choose the grid columns

Click **Columns** to choose which columns the grid shows and in what order. Tick a column to show it and use the ▲ and ▼ buttons to move it.

![The Columns dialog](images/browse-05-columns.png)

**Export** saves the tests the grid currently shows as CSV or XLSX.

## Read and edit a test

1. Click a row in the grid. The detail panel opens on the right.

   ![The detail panel for QA-4998](images/browse-06-detail.png)

   The header shows the key, the status and two buttons: **Clone** copies the test, and **✕** closes the panel. Clicking the key opens the test in Jira in your browser.

2. Edit any field in place: Summary, Priority, Labels, Components, Execution type or Folder. To change the status, pick a transition from **Move to…**. There is no Save button. XTM saves the change locally when you leave the field.

   An edited field gets an orange dot, the row in the grid gets the same dot, and the **N pending** button in the top bar counts the change.

   ![The summary edited, with the dot on the field, the row and the pending count](images/browse-08-edited-field.png)

3. Scroll down for the rest of the test:
   - **Custom fields**.
   - **Preconditions**: **+ Add precondition…** links one, **+ New** creates one.
   - **Memberships**: the sets, plans and executions the test belongs to.
   - **Requirements**.
   - **Bugs**: linked defects.
   - **Run history**: results from every execution that ran the test.
   - **Description**: click to edit. Markdown is supported.
   - **Steps**.
   - **Dataset**: the parameters the test runs with, if it has any.

   ![The Description and Steps sections](images/browse-07-detail-steps.png)

### Edit steps

Each step has an action, optional data, and an expected result. Type in a box to change it.

- **+ Add step** adds a step at the end.
- **+ Call test** adds a step that calls another test. See [Test Calls](06-test-calls.md).
- The arrows move a step up or down, the copy button duplicates it, and **✕** deletes it.
- **Clone from…** copies the steps of another test.
- **Refresh** reloads the steps from Jira.

When the test has a dataset, its Action, Data and Expected fields have a
**${** button. Click it, or type `${` in the field. Pick a parameter with the
mouse, or with the arrow keys and Enter, and XTM inserts the full
`${PARAMETER}` macro at the caret. A reference that matches no dataset
parameter stays in the text, with an error below the field.

Cucumber tests show a **Cucumber scenario** box instead of steps, and Generic tests show a **Generic definition** box.

### Give a test a dataset

A dataset is the table that a test's `${placeholder}` references read their values from. It has one column per parameter and one row per iteration, and it sits below the steps.

- A test with no dataset shows **Create dataset**, and a test with one shows **Edit**. Both open the dataset dialog. Parameter and file actions sit above the table, and Save and Cancel are in the footer.
- Each column is a parameter. Click **Add parameter**, or open **Actions** in an existing column header, to set its name, type and combinatorial behavior. The same menu moves or removes the column along with its values.
- Choose **Text** for free-form values. Choose **List** to enter an ordered set of options, one per line. List cells become dropdowns, and XTM refuses to remove an option while a row still uses it.
- **Add row** adds an iteration, and **Remove** at the end of a row deletes that row.
- When you open a test from a Test Execution, you see the dataset that will run there and whether it belongs to the test or the execution. XTM shows an execution's own dataset as read-only.
- XTM keeps a parameter that uses a predefined Jira list as it is. Its binding and options stay read-only until you convert it to an ad-hoc list. XTM does not manage project lists.
- Below the dataset, **Iterations** shows how each row did in that execution: the values it ran with, its result, and how many rows passed. A test with a dataset runs once per row, and the run's overall result comes from those row results. Look here to see which values failed. The section appears only when the run has iterations.
- The arrows at the end of a row move that row up or down. Row order is the order the iterations run in.
- **Export CSV** saves names and values in Xray's CSV layout. **Import CSV** replaces the table on screen but stores nothing until **Save**. CSV has no field for parameter type, options or Jira ids. A column that matches an existing parameter by name keeps its List metadata, and a new column becomes Text.
- When you rename a parameter, XTM asks whether to rewrite `${OLD}` to `${NEW}` in the test's steps and definition. Yes turns each rewritten field into its own pending change, so you can review them before they go to Jira. No leaves the text as it was. Choose no when the existing references never pointed at this parameter.
- **Save** stores the table locally and counts it in **N pending**, like every other edit. Jira gets it on the next commit.

XTM checks Xray's limits when you save: at most 20 parameters, at most 100 characters in a value, no two parameters with the same name, no two identical rows, and names that start with a letter or underscore. A List also needs unique options, and every cell must use one of them. If the table breaks a rule, XTM refuses it, gives the reason, and leaves the table on screen for you to fix.

Saving a dataset to Jira replaces the whole table. If someone changed it in Jira after you loaded the test, the commit stops and asks which version to keep, so their change is not overwritten.

## Create a test

1. Select the folder the test belongs in. XTM fills in the folder for you.
2. Click **＋ New Test** in the top bar. This button only shows on the Browse view.
3. Fill in **Summary** (the only required field), then any of Description, Priority, Folder, Execution type, Labels and Components. You can add steps with **＋ add step** and link preconditions.

   ![The New Test panel](images/browse-09-new-test.png)

4. Click **Create →**.

The new test gets a temporary key such as `NEW-5`, and a banner says it has not been created in Jira yet. Jira creates it, with a real key, when you commit.

![The new test, waiting to be committed](images/browse-10-new-test-created.png)

## Change many tests at once

1. Tick the checkbox of each test you want to change. The checkbox in the header selects every test on the page.
2. A yellow toolbar appears under the view tabs with the number selected and the bulk actions.

   ![The bulk toolbar with three tests selected](images/browse-11-bulk-toolbar.png)

3. Pick an action:
   - **Bulk edit…** sets one field on every selected test.
   - **Labels…** and **Components…** add or remove values.
   - **Rename summaries…** finds and replaces text in the summaries.
   - **Bulk transition…** moves every selected test through a workflow transition.
   - **Allocate…** adds the tests to a Test Set, Plan or Execution.
   - **Move to folder…** moves them to another folder.
   - **Preconditions…** and **Requirements…** link or unlink in bulk.

   Some actions only appear when the backend supports them.

4. Confirm in the dialog. **Bulk edit** shows how many tests it will change before you click **Apply**.

   ![Bulk edit for three tests](images/browse-12-bulk-edit.png)

**Clear** in the toolbar deselects everything. Bulk changes are pending changes, so you review and commit them like any other edit.

## Import tests from CSV or XLSX

1. Choose **More → Import tests…** (or **File → Import Tests…**).

   ![The Import tests dialog](images/browse-13-import.png)

2. Click **Choose File** and pick a CSV or XLSX file. **Download template** saves a sample file with every column XTM understands.

3. Check the mapping. XTM matches columns whose header is the field name, such as `Summary`, `Priority` or `Folder`. Pick a column for any field it did not match, or leave it as **(not mapped)**. Summary is required.

   ![Columns mapped to fields](images/browse-14-import-mapping.png)

4. Click **Validate**. XTM reports how many rows are valid and lists any problems.

   ![The validation result](images/browse-15-import-validated.png)

5. Click **Import**. The tests become pending creates. Commit them from the Pending list.

   ![The import result](images/browse-16-import-done.png)

## Troubleshooting

- **A test you expect is missing.** Check the folder selection, the search box and the status and exec type filters. Then sync, because the test may be newer than your cache.
- **New Test is not in the top bar.** It only shows on the Browse view.
- **Columns is missing.** It is hidden while the detail panel is open. Close the panel.
- **Import rejects a file.** Make sure the file has a `Summary` column and that the first row is the header row.
