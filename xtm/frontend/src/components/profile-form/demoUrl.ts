// isXtmDemoUrl is what XTM accepts as a demo base URL: "demo", a "demo:" or
// "mock:" prefix, a "demo-" variant naming a built-in dataset (demo-pkcs), and
// the offline Kiwi demo in the same forms. Keep in sync with isDemoURL in
// internal/jira/demo.go and kiwi.IsKiwiDemoURL.
export const isXtmDemoUrl = (u: string) => /^(demo|demo[-:].*|mock:.*|kiwi-demo|kiwi-demo[-:].*)$/i.test(u);
