import { describe as group, expect, it } from "vitest";
import type { AuditEntry } from "../api";
import { describe } from "./ActivityTab";

const entry = (action: string, field: string): AuditEntry => ({
  id: 1, occurredAt: "", actor: "Demo User", entityType: "link", entityKey: "PLAT-1", action, field, beforeVal: "", afterVal: "", note: "",
});

group("a link removal in the activity (#612)", () => {
  it("says the link was removed, pushed, or kept", () => {
    expect(describe(entry("unlink", "-|Blocks|outward|PLAT-2"))).toBe("Demo User removed a link: Blocks (outward) to PLAT-2");
    expect(describe(entry("commit", "-|Blocks|outward|PLAT-2"))).toBe("Demo User pushed the removal of the link Blocks (outward) to PLAT-2");
    expect(describe(entry("discard", "-|Blocks|outward|PLAT-2"))).toBe("Demo User discarded the removal of the link Blocks (outward) to PLAT-2");
    expect(describe(entry("link", "Blocks|outward|PLAT-2"))).toBe("Demo User added a link: Blocks (outward) to PLAT-2");
  });
});
