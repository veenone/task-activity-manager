# Changelog

All notable changes to **Project Issue Explorer** are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The version is single-sourced in `wails.json` (`info.productVersion`).

## [Unreleased]

## [0.1.0] - 2026-11-09

The first release. PIE reads Jira and never writes to it. The only thing it
writes outside your computer is the Confluence page for a cycle you publish.

### Added

**Queries**
- A profile per Jira instance, with the token in the operating system's
  credential store (#381). A demo profile with seeded data needs no Jira (#390).
- Run a JQL filter across projects, then save it and keep its last run. PIE
  opens with the last run's rows and says which issues came or went since the
  run before (#394).
- Start from a starred or numbered Jira filter (#404). Fold the JQL field away
  once the query is set (#414).
- Results filters by project, status and priority (#400), long lists in pages,
  and project names next to their keys (#398).
- Name the client visibility, client impact and product base fields for each
  profile, and read an issue's details beside the list (#396, #402).

**Looking across projects**
- Trace linked issues across projects and see which sets are out of step:
  some issues finished and some still open (#392).
- Statistics over a run (#396).
- A Project map as a table, a flow and a tree (#418), with product baselines
  you map by hand (#416). A Link map by link type (#419).

**Cases**
- Open cases from Results rows (#406). A case page lists the projects to
  assess, each with a verdict and a reason (#408).
- Find evidence for a case, then accept or dismiss it (#410).
- A grid of cases against projects (#412).

**Reporting**
- Cycles keep a copy of every case and its verdicts, and show what changed
  since the last published cycle (#423).
- Publish a cycle to Confluence under a page you choose. PIE asks before it
  takes over a page it did not write, or writes over an edit made in
  Confluence (#425).
