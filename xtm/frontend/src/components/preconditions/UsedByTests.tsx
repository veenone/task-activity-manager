import { Pager } from "../Pager";
import type { PreconditionsState } from "./usePreconditionsState";
import type { PreconditionsActions } from "./usePreconditionsActions";

// UsedByTests is the paged table of tests that reference the precondition.
export function UsedByTests({ s, a }: { s: PreconditionsState; a: PreconditionsActions }) {
  const { setShowAdd, detailKey, setDetailKey, setTestsPage, testsPageSize, setTestsPageSize, tests, testsSafePage, pageTests } = s;
  const { removeTest } = a;
  return (
    <>
      <div className="precond-tests-head">
        <h4>
          Used by {tests.length} test{tests.length === 1 ? "" : "s"}
        </h4>
        <button
          className="btn"
          onClick={() => setShowAdd(true)}
          title="Link tests to this precondition"
        >
          + Add tests
        </button>
      </div>

      {tests.length === 0 ? (
        <p className="muted">No tests reference this precondition yet.</p>
      ) : (
        <>
          <table aria-label="Tests using this precondition" className="board-table precond-tests">
            <thead>
              <tr>
                <th scope="col">Test</th>
                <th scope="col">Summary</th>
                <th scope="col">Status</th>
                <th scope="col" aria-label="Remove" />
              </tr>
            </thead>
            <tbody>
              {pageTests.map((t) => (
                <tr
                  key={t.key}
                  className={
                    t.key === detailKey ? "precond-test-row-active" : ""
                  }
                >
                  <td>
                    <button
                      className="link-btn mono"
                      onClick={() => setDetailKey(t.key)}
                      title="Open test detail"
                    >
                      {t.key}
                    </button>
                  </td>
                  <td>{t.summary}</td>
                  <td>{t.status || "(none)"}</td>
                  <td className="board-remove-cell">
                    <button
                      className="btn btn-ghost board-remove"
                      onClick={() => removeTest(t.key)}
                      title="Unlink from this precondition"
                      aria-label="Unlink from this precondition"
                    >
                      ✕
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {tests.length > testsPageSize && (
            <Pager
              page={testsSafePage}
              pageSize={testsPageSize}
              total={tests.length}
              onPage={setTestsPage}
              onPageSize={(n) => {
                setTestsPageSize(n);
                setTestsPage(0);
              }}
            />
          )}
        </>
      )}
    </>
  );
}
