# TAM Phase 5: Confluence foundation

## Goal

Add the configuration and read-only API foundation needed by the Phase 5
Rituals view. This slice does not render pages or add caching; it establishes
the profile settings, credential boundary, normalized page model, and tested
HTTP client that later UI work will consume.

## Profile and credential boundary

Each profile gains optional Confluence settings:

- `confluenceUrl`: the Confluence Data Center base URL.
- `confluenceSpace`: the space key used by Rituals.
- `confluenceRootPageId`: an optional root page whose descendants are ritual documents.

The Confluence token is stored in the OS credential manager under a key derived
from the profile ID and a Confluence-specific suffix. It is never stored in the
SQLite profile row, frontend profile JSON, or exported profile configuration.
Existing Jira credentials and profile behavior remain unchanged. Empty
Confluence settings mean Confluence is not configured.

TLS behavior follows the profile's existing CA certificate and
`allowUntrustedTls` settings.

## Confluence client

Create `core/confluence` with a small interface and HTTP implementation. The
client supports:

- fetching one page by ID;
- listing direct child pages for a parent page, with pagination;
- decoding page ID, title, space key, version/update metadata, web URL, and
  rendered HTML body.

Requests use the Confluence REST API, send the token in the Authorization
header, and normalize the configured base URL before joining paths. The client
returns typed errors for unauthorized/forbidden, not found, rate limited, and
transport failures while preserving the HTTP status for callers and tests.

## App seam

Add App methods for reading and writing the Confluence profile settings and for
fetching/listing pages through the client. App methods validate the profile,
load the Confluence credential from the credential store, apply the profile TLS
settings, and return normalized models suitable for Wails bindings.

No page editing, comments, attachments, full-text search, or local page cache
is included in this slice.

## Verification

- Profile tests cover round-tripping optional Confluence fields and ensuring
  exported profile configuration excludes the token.
- Client tests use an `httptest` server to verify auth headers, URL joining,
  page decoding, child-page pagination, and status/error mapping.
- App tests cover missing configuration, missing credentials, and successful
  page reads.
- Existing core and TAM test suites, frontend typecheck, and build remain
  green.
