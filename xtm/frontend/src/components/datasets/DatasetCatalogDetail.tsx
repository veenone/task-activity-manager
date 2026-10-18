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

function Definition({ definition }: { definition: DatasetCatalogDefinition }) {
  return (
    <div className="dataset-definition">
      {definition.contextKey && (
        <div className="dataset-definition-title">
          <strong>{definition.contextKey}</strong>
          {definition.contextSummary && <span>{definition.contextSummary}</span>}
        </div>
      )}
      {!definition.metadataComplete && (
        <p className="dataset-unknown">Parameter metadata unknown for Plan datasets.</p>
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
    </div>
  );
}

function UsageLink({
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
    <li>
      <button
        className="link-btn"
        aria-label={`Open ${label} ${usage.containerKey}`}
        onClick={() => onOpen({ kind, key: usage.containerKey })}
      >
        {usage.containerKey}
      </button>
      <span>{usage.containerSummary}</span>
      <span className={overridden ? "dataset-override" : "muted"}>
        {overridden ? "Override" : "Inherited default"}
      </span>
    </li>
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
  if (query.isLoading) return <p className="muted">Loading dataset details…</p>;
  if (query.error) return <p className="error-text">Unable to load dataset details.</p>;
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
      <section>
        <h3>Default dataset</h3>
        {defaults.length > 0 ? (
          defaults.map((definition) => (
            <Definition key="default" definition={definition} />
          ))
        ) : (
          <p className="muted">No Test-level default.</p>
        )}
      </section>
      {plans.length > 0 && (
        <section>
          <h3>Plan overrides</h3>
          {plans.map((definition) => (
            <Definition key={definition.contextKey} definition={definition} />
          ))}
        </section>
      )}
      {executions.length > 0 && (
        <section>
          <h3>Execution overrides</h3>
          {executions.map((definition) => (
            <Definition key={definition.contextKey} definition={definition} />
          ))}
        </section>
      )}
      <section className="dataset-usages">
        <h3>Used by</h3>
        {usages.length === 0 ? (
          <p className="muted">No synced Plan or Execution uses this Test.</p>
        ) : (
          <ul>
            {usages.map((usage) => {
              const scope = usage.containerKind === "testplan" ? "plan" : "execution";
              return (
                <UsageLink
                  key={`${usage.containerKind}:${usage.containerKey}`}
                  usage={usage}
                  overridden={overrideKeys.has(`${scope}:${usage.containerKey}`)}
                  onOpen={onOpenContainer}
                />
              );
            })}
          </ul>
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
