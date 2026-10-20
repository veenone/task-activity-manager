# Xray Test Plan dataset probe

## Check status

No connected Xray instance was available on 18 October 2026. This record pins
the transport using the checked-in Xray WADL and response fixtures captured
during the earlier dataset spike. A live Plan override and inherited membership
still need verification before merge.

## Request contract

The WADL at
`xtm/internal/jira/testdata/dataset_export_v2.wadl.xml` defines this request:

```text
GET /rest/raven/2.0/api/dataset/export?contextIssueKey=PLAN-1&resolved=false&testIssueKey=TEST-1
Accept: application/octet-stream
```

The client must send `resolved=false`. The route defaults to resolved output,
which may return a Test dataset inherited by the Plan.

## Override response

The WADL declares `application/octet-stream`. The captured export fixtures use
raw CSV, including the asterisk suffix for combinatorial parameters. This is a
sanitized example of the body shape:

```csv
PARAMETER_1,PARAMETER_2*
VALUE_1_1,VALUE_1_2
```

The fixture-backed contract expects status 200 and parses the body with
`ParseDatasetCSV`. CSV does not carry parameter IDs, list type, list ID, list
scope, or allowed values, so those fields remain unknown.

## Inherited membership response

`xtm/internal/jira/testdata/dataset_export_absent.json` records the export
route's dataset-absence response:

```text
Status: 404
Content-Type: application/json
Body: {"error":"Dataset not found"}
```

Until the live Plan check is possible, the client treats that exact response as
an inherited membership with no Plan definition. A different 404 remains an
error. The checked-in missing-route fixture is Jersey XML:

```text
Status: 404
Content-Type: application/xml
Body: <status><status-code>404</status-code></status>
```

Keeping these cases separate prevents an unsupported Xray route from producing
a successful but empty catalog.

## Live check still required

Run the same read-only request against one Plan override and one inherited Plan
membership when a connected Xray Server or Data Center profile is available.
Record the status, content type, and sanitized body for both responses. If the
inherited response differs from the captured absence fixture, update the client
contract and its tests before merging.
