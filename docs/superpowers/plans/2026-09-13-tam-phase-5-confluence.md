# TAM Phase 5: Confluence and Rituals completion

This plan records the shipped Phase 5 work and its follow-up repairs.

- Keep Confluence settings in the shared profile database and its token in the
  OS credential store.
- Expose page reads and child-page pagination through the app seam.
- Render linked Rituals pages, sanitize storage HTML, and cache page payloads.
- Purge Confluence configuration, associations, cached pages, and credentials
  when a profile is deleted.
- Verify URL joining, authorization, page decoding, pagination, error mapping,
  profile round-trips, and export behavior without exporting secrets.
