// A path is one issue, or one link, through the layers of a flow: the box it
// sits in on each layer, left to right (#473). type, when given, splits the
// lines between two boxes by it, as the Link map does by link type.
export type FlowPath = { key: string; steps: string[]; type?: string };
export type FlowBox = { id: string; column: number; keys: string[] };
export type FlowLine = {
  id: string;
  source: string;
  target: string;
  type: string;
  keys: string[];
};

// layered groups the paths into one line per pair of boxes on neighbouring
// shown layers. A hidden layer is skipped, so its neighbours join directly.
// A line with fewer keys than min is dropped, and so is a box left without
// a line. Boxes come back layer by layer, in the order the paths met them.
export function layered(
  paths: FlowPath[],
  { hidden, min }: { hidden: Set<number>; min: number },
) {
  const lines = new Map<string, FlowLine>();
  for (const p of paths) {
    const shown = p.steps.flatMap((id, column) =>
      hidden.has(column) ? [] : [{ id, column }],
    );
    for (let i = 1; i < shown.length; i += 1) {
      const [source, target, type] = [
        shown[i - 1].id,
        shown[i].id,
        p.type ?? "",
      ];
      const id = `${source}|${target}|${type}`;
      const line = lines.get(id) ?? { id, source, target, type, keys: [] };
      if (!line.keys.includes(p.key)) line.keys.push(p.key);
      lines.set(id, line);
    }
  }
  const kept = [...lines.values()].filter((l) => l.keys.length >= min);
  const column = new Map(
    paths.flatMap((p) => p.steps.map((id, c) => [id, c] as const)),
  );
  const boxes = new Map<string, FlowBox>();
  for (const l of kept)
    for (const id of [l.source, l.target]) {
      const box = boxes.get(id) ?? {
        id,
        column: column.get(id) ?? 0,
        keys: [],
      };
      for (const k of l.keys) if (!box.keys.includes(k)) box.keys.push(k);
      boxes.set(id, box);
    }
  return {
    boxes: [...boxes.values()].sort((a, b) => a.column - b.column),
    lines: kept,
  };
}
