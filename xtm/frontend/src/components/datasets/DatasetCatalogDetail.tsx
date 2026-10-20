import { BrowserOpenURL, isDemoUrl } from "../../api";
import type {
  DatasetCatalogDefinition,
  DatasetCatalogUsage,
} from "../../api";
import { useDatasetCatalogDetail } from "../../queries/datasets";
import type { ContainerNavigationTarget } from "../../contexts/NavContext";

interface Props {
  profileId: string;
  testKey: string;
  open: boolean;
  jiraUrl: string;
  onOpenTest: (key: string) => void;
  onOpenContainer: (target: ContainerNavigationTarget) => void;
}

function Definition({
  definition,
  label,
}: {
  definition: DatasetCatalogDefinition;
  label: string;
}) {
  return (
    <section className="dataset-definition" aria-label={label}>
      {definition.contextKey && (
        <div className="dataset-definition-title">
          <strong>{definition.contextKey}</strong>
          {definition.contextSummary && <span>{definition.contextSummary}</span>}
        </div>
      )}
      {!definition.metadataComplete && (
        <p className="dataset-unknown">Plan datasets do not say what type each parameter is.</p>
      )}
      <div className="dataset-parameter-list">
        {definition.parameters.map((parameter) => (
          <span key={parameter.name} className="dataset-parameter">
            {parameter.name}{parameter.combinatorial ? " ×" : ""}
          </span>
        ))}
        <span className="muted">
          {definition.rows.length} row{definition.rows.length === 1 ? "" : "s"}
        </span>
      </div>
    </section>
  );
}

function UsageRow({
  usage,
  overridden,
  onOpen,
}: {
  usage: DatasetCatalogUsage;
  overridden: boolean;
  onOpen: (target: ContainerNavigationTarget) => void;
}) {
  const kind = usage.containerKind === "testplan" ? "testplan" : "testexec";
  const label = kind === "testplan" ? "Plan" : "Execution";
  return (
    <tr>
      <td>{label}</td>
      <td>
        <button
          className="link-btn"
          aria-label={`Open ${label} ${usage.containerKey}`}
          onClick={() => onOpen({ kind, key: usage.containerKey })}
        >
          {usage.containerKey}
        </button>
      </td>
      <td className="dataset-usage-summary">{usage.containerSummary}</td>
      <td>
        <span className={overridden ? "dataset-override" : "muted"}>
          {overridden ? "Override" : "Inherited default"}
        </span>
      </td>
    </tr>
  );
}

export function DatasetCatalogDetail({
  profileId,
  testKey,
  open,
  jiraUrl,
  onOpenTest,
  onOpenContainer,
}: Props) {
  const query = useDatasetCatalogDetail(profileId, testKey, open);
  if (!open) return null;
  if (query.isLoading) return <p className="muted">Loading the dataset…</p>;
  if (query.error) return <p className="error-text">Could not load the dataset.</p>;
  const detail = query.data;
  if (!detail) return null;

  const defaults = detail.definitions.filter((item) => item.scopeKind === "test");
  const plans = detail.definitions.filter((item) => item.scopeKind === "plan");
  const executions = detail.definitions.filter(
    (item) => item.scopeKind === "execution",
  );
  const overrideKeys = new Set(
    detail.definitions
      .filter((item) => item.scopeKind !== "test")
      .map((item) => `${item.scopeKind}:${item.contextKey}`),
  );
  const usages = [
    ...new Map(
      detail.usages.map((usage) => [
        `${usage.containerKind}:${usage.containerKey}`,
        usage,
      ]),
    ).values(),
  ];
  const jiraBase = jiraUrl.trim().replace(/\/+$/, "");

  return (
    <div className="dataset-detail-grid">
      <div className="dataset-scope-group">
        <h3>Default dataset</h3>
        {defaults.length > 0 ? (
          defaults.map((definition) => (
            <Definition
              key="default"
              definition={definition}
              label="Default dataset"
            />
          ))
        ) : (
          <p className="muted">No Test-level default.</p>
        )}
      </div>
      {plans.length > 0 && (
        <div className="dataset-scope-group">
          <h3>Plan overrides</h3>
          {plans.map((definition) => (
            <Definition
              key={definition.contextKey}
              definition={definition}
              label={`Plan ${definition.contextKey} dataset`}
            />
          ))}
        </div>
      )}
      {executions.length > 0 && (
        <div className="dataset-scope-group">
          <h3>Execution overrides</h3>
          {executions.map((definition) => (
            <Definition
              key={definition.contextKey}
              definition={definition}
              label={`Execution ${definition.contextKey} dataset`}
            />
          ))}
        </div>
      )}
      <section className="dataset-usages">
        <h3>Used by</h3>
        {usages.length === 0 ? (
          <p className="muted">No synced Plan or Execution uses this Test.</p>
        ) : (
          <div className="dataset-usages-scroll">
            <table
              className="dataset-usage-table"
              aria-label="Containers using this dataset"
            >
              <thead>
                <tr>
                  <th scope="col">Type</th>
                  <th scope="col">Container</th>
                  <th scope="col">Summary</th>
                  <th scope="col">Dataset</th>
                </tr>
              </thead>
              <tbody>
                {usages.map((usage) => {
                  const scope = usage.containerKind === "testplan" ? "plan" : "execution";
                  return (
                    <UsageRow
                      key={`${usage.containerKind}:${usage.containerKey}`}
                      usage={usage}
                      overridden={overrideKeys.has(`${scope}:${usage.containerKey}`)}
                      onOpen={onOpenContainer}
                    />
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
      <div className="dataset-detail-actions">
        {detail.test.internalOpenable && (
          <button className="btn" onClick={() => onOpenTest(testKey)}>
            Open Test {testKey}
          </button>
        )}
        {jiraBase && !isDemoUrl(jiraUrl) && (
          <button
            className="btn btn-ghost"
            onClick={() => BrowserOpenURL(`${jiraBase}/browse/${testKey}`)}
          >
            Open {testKey} in Jira
          </button>
        )}
      </div>
    </div>
  );
}
