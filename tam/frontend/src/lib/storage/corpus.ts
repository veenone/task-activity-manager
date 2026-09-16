// Test data only: the pages every storage round-trip test runs over, shared
// so the plain parse/serialize check (storage.test.ts) and the check through
// the editor's real schema (schemaRoundTrip.test.ts) cover the same pages.
// Nothing outside a test imports this.
import sprint from "../../../../internal/ritualtemplate/testdata/sprint.xml?raw";
import planning from "../../../../internal/ritualtemplate/testdata/planning.xml?raw";
import standup from "../../../../internal/ritualtemplate/testdata/standup.xml?raw";
import review from "../../../../internal/ritualtemplate/testdata/review.xml?raw";
import retro from "../../../../internal/ritualtemplate/testdata/retro.xml?raw";

export const templates: Record<string, string> = { sprint, planning, standup, review, retro };

export const handWritten: Record<string, string> = {
  layout: `<ac:layout><ac:layout-section ac:type="two_equal"><ac:layout-cell><p>Left</p></ac:layout-cell><ac:layout-cell><p>Right</p></ac:layout-cell></ac:layout-section></ac:layout>`,
  codeMacro: `<p>Before</p><ac:structured-macro ac:name="code" ac:schema-version="1" ac:macro-id="abc"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><![CDATA[if a < b && c > d { return }]]></ac:plain-text-body></ac:structured-macro><p>After</p>`,
  mention: `<p>Ask <ac:link><ri:user ri:userkey="8a7f808a"/></ac:link> about <strong>infra</strong> access.</p>`,
  colouredSpan: `<p>Status is <span style="color: rgb(255,0,0);">red</span> today.</p>`,
  mergedCells: `<table><colgroup><col/><col/></colgroup><tbody><tr><th colspan="2">Capacity</th></tr><tr><td>Dian</td><td><p>8 days</p></td></tr></tbody></table>`,
  entities: `<p>Owner&nbsp;notes &mdash; keep &amp; share &lt;tags&gt;</p>`,
  nestedTasks: `<ac:task-list><ac:task><ac:task-id>1</ac:task-id><ac:task-status>complete</ac:task-status><ac:task-body>Parent<ac:task-list><ac:task><ac:task-id>2</ac:task-id><ac:task-status>incomplete</ac:task-status><ac:task-body>Child</ac:task-body></ac:task></ac:task-list></ac:task-body></ac:task></ac:task-list>`,
  image: `<p><ac:image ac:height="250"><ri:attachment ri:filename="board.png"/></ac:image></p>`,
  emoticon: `<p>Shipped <ac:emoticon ac:name="tick"/></p>`,
  mixedList: `<ul><li>Item<ul><li>Sub item</li></ul></li><li><p>Para item</p></li></ul>`,
  bareRoot: `Loose text <em>here</em>`,
  inlineMacro: `<p>State: <ac:structured-macro ac:name="status"><ac:parameter ac:name="colour">Green</ac:parameter><ac:parameter ac:name="title">On track</ac:parameter></ac:structured-macro></p>`,
  marksAndLinks: `<p>a <strong>b <em>c</em></strong> <a href="https://example.com" title="x">d</a><br/>e <s>gone</s> <u>under</u> <code>x()</code></p>`,
  headingsAndRule: `<h1>One</h1><h4 style="text-align: center;">Four</h4><hr/><blockquote><p>Quoted</p></blockquote>`,
  // A shape parse.ts cannot hold exactly must stay opaque rather than drop
  // or rewrite content. Round 1 fix: an attribute on any of the four task
  // elements, a task-id or task-status with anything but a single text node
  // (or an empty task-id), a status that is not exactly "complete" or
  // "incomplete", and an unknown element before task-id all keep the whole
  // task list opaque; a properly empty task-id still round-trips.
  taskAttrs: `<ac:task-list><ac:task data-x="1"><ac:task-id>1</ac:task-id><ac:task-status>incomplete</ac:task-status><ac:task-body><p>x</p></ac:task-body></ac:task></ac:task-list>`,
  taskIdAttrs: `<ac:task-list><ac:task><ac:task-id data-x="1">1</ac:task-id><ac:task-status>incomplete</ac:task-status><ac:task-body><p>x</p></ac:task-body></ac:task></ac:task-list>`,
  taskStatusAttrs: `<ac:task-list><ac:task><ac:task-id>1</ac:task-id><ac:task-status data-x="1">incomplete</ac:task-status><ac:task-body><p>x</p></ac:task-body></ac:task></ac:task-list>`,
  taskBodyAttrs: `<ac:task-list><ac:task><ac:task-id>1</ac:task-id><ac:task-status>incomplete</ac:task-status><ac:task-body data-x="1"><p>x</p></ac:task-body></ac:task></ac:task-list>`,
  taskEmptyId: `<ac:task-list><ac:task><ac:task-id/><ac:task-status>incomplete</ac:task-status><ac:task-body><p>x</p></ac:task-body></ac:task></ac:task-list>`,
  taskStatusPadded: `<ac:task-list><ac:task><ac:task-id>1</ac:task-id><ac:task-status> complete </ac:task-status><ac:task-body><p>x</p></ac:task-body></ac:task></ac:task-list>`,
  taskStatusUnknown: `<ac:task-list><ac:task><ac:task-id>1</ac:task-id><ac:task-status>unknown</ac:task-status><ac:task-body><p>x</p></ac:task-body></ac:task></ac:task-list>`,
  taskIdComment: `<ac:task-list><ac:task><ac:task-id><!--c--></ac:task-id><ac:task-status>incomplete</ac:task-status><ac:task-body><p>x</p></ac:task-body></ac:task></ac:task-list>`,
  taskIdOutOfOrder: `<ac:task-list><ac:task><ac:task-uuid>u</ac:task-uuid><ac:task-id>1</ac:task-id><ac:task-status>incomplete</ac:task-status><ac:task-body><p>x</p></ac:task-body></ac:task></ac:task-list>`,
  // Round 1 fix: colspan/rowspan only lift into the numeric attr when the
  // raw value is a plain positive integer other than "1"; otherwise the
  // raw text stays in extra, untouched, rather than being lost or rewritten.
  cellColspanOne: `<table><tbody><tr><td colspan="1">a</td><td>b</td></tr></tbody></table>`,
  cellColspanZero: `<table><tbody><tr><td colspan="0">a</td><td>b</td></tr></tbody></table>`,
  cellColspanNaN: `<table><tbody><tr><td colspan="abc">a</td><td>b</td></tr></tbody></table>`,
  cellColspanPadded: `<table><tbody><tr><td colspan=" 2">a</td><td>b</td></tr></tbody></table>`,
  // Round 1 fix: a mark element or a[href] with no child nodes goes to
  // opaqueInline rather than being dropped along with its tags.
  emptyStrong: `<p>a<strong></strong>b</p>`,
  emptyLink: `<p>a<a href="#x"></a>b</p>`,
  // Round 1 fix: stripNamespaces must only touch start tags, never text or
  // CDATA content that happens to look like a namespace declaration.
  xmlnsInCdata: `<p>Before</p><ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[<ac:x xmlns:ac="http://atlassian.com/content"/>]]></ac:plain-text-body></ac:structured-macro><p>After</p>`,
  xmlnsLookalikeText: `<p><span style="x">write xmlns:ac="u" here</span></p>`,
};

export const corpus: Record<string, string> = { ...templates, ...handWritten };
