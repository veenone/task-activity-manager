// Extends Vitest's expect with jest-dom matchers (toBeInTheDocument, etc.).
import "@testing-library/jest-dom/vitest";

import { afterEach } from "vitest";
import { cleanup, configure } from "@testing-library/react";

// findBy* waits 1000ms by default, which is generous on an idle machine and
// not always enough when fourteen test files run in parallel: a query that
// waits on a resolved promise plus a re-render (the sub-task button waits on
// GetSubtaskTypeName) can miss that window under load and fail a suite that
// passes serially. The same reason vite.config.ts raises testTimeout.
configure({ asyncUtilTimeout: 5000 });

// Testing Library only registers its own afterEach cleanup when Vitest globals
// are enabled, and this project keeps them off (vite.config.ts sets no
// `globals`). Without this, every render stays mounted for the rest of the
// file, so a second test rendering the same component sees two of everything.
// Registered here rather than per file so component tests get it for free.
afterEach(cleanup);

// jsdom implements no layout, so Element.scrollIntoView is undefined and any
// component that scrolls a row into view throws under test. A no-op keeps the
// behaviour under test (which row was scrolled to is not asserted anywhere)
// without each suite stubbing it again.
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {};
}
