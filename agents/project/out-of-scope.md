# Out of scope

Requests the maintainer has declined, with the reason, so nobody re-opens
or re-proposes them without new evidence. Check this before opening an
issue or suggesting a feature (P1). One entry per decision: the gist, why,
and the issue that records it. An entry is reopened only by a new issue
that cites the old one and says what changed.

- Rendering a Jira dashboard's gadgets in TAM. Reading the gadgets on a
  dashboard needs `GET /dashboard/{dashboardId}/gadget`, which is Cloud only
  and absent from the Data Center 9.12 reference. The property routes,
  `/dashboard/{dashboardId}/items/{itemId}/properties` and
  `.../properties/{propertyKey}`, are present on Data Center but need an
  `itemId` that only the Cloud route hands out, so they are unreachable in
  practice. Gadget property keys are undocumented and differ per gadget type
  as well: `config` 404s on a Bubble Chart, and a property listing on another
  gadget answered with only `itemkey`. TAM lists Jira's dashboards as a
  signpost and opens them in a browser instead (#349).
- Feature suggestions made without first checking the existing surface.
  Grep the code and the user docs before proposing; an inventory from a
  subagent alone misses things.
