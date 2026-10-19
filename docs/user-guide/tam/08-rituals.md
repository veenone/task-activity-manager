# Rituals

A team's ceremonies usually leave a trail of Confluence pages: planning notes, a standup log, review notes, a retrospective. The Rituals view keeps that set per sprint, edits it in TAM, and syncs it with Confluence.

It needs the **Confluence Rituals** settings on the profile. Without them the view says so and does nothing else.

## What a sprint has

Pick a **Board** and a **Sprint**, and the documents appear:

| Page | What it holds |
|---|---|
| Overview | The sprint's own page, which the others sit under |
| Planning | What the team took on and why |
| Standup | The daily log |
| Review | What was shown |
| Retrospective | What the team decided to change |

Alongside them is the **Done agreement**: the team's definition of done, kept at board level, with any additions this sprint made. It also appears in the issue detail panel, so it is readable while an issue is.

A sprint that closed before the team used this view says so rather than offering to create pages for it after the fact.

## Editing

The editor is a rich text field with headings, bullet and numbered lists, tables and links. A Confluence macro the editor cannot safely change, such as a Jira issue list, is shown as it renders and kept exactly as it is; edit those in Confluence.

On the Standup page, **Add today's entry** adds a dated heading under the daily log. It refuses politely when today already has one, or when the Daily log heading has been removed.

What you type is local. Nothing reaches Confluence until you press **Sync rituals**, which is the one control in this view that talks to a server.

## Sync

**Sync rituals** creates the pages a sprint does not have yet, sends what you edited, and brings back what changed in Confluence. It reports what it did, including when everything was already in step.

**Open in Confluence** opens the page in your browser.

### When both sides changed

A page edited here and in Confluence since the last sync is held as a conflict, with two choices:

- **Keep mine** sends your version.
- **Take theirs** drops your edit and takes the Confluence version.

### When a page is gone

A page deleted or moved in Confluence is marked **Gone from Confluence**. You can **Recreate on next Sync**, which puts it back under the current root, or **Remove local copy** to forget it.

## The root page

Every ritual page lives under a root page in a Confluence space, set in the profile. Changing the root re-syncs straight afterwards: pages the sprint does not have yet are created under the new root, and a page left behind under the old one shows as gone, with the option to recreate it where the new root is.
