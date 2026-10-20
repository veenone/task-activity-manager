export const DATASET_ROW_COLUMN_WIDTH = 48;
export const DATASET_PARAMETER_COLUMN_WIDTH = 180;
export const DATASET_ACTIONS_COLUMN_WIDTH = 150;

export function datasetEditorMinWidth(parameterCount: number): number {
  return DATASET_ROW_COLUMN_WIDTH + DATASET_ACTIONS_COLUMN_WIDTH + parameterCount * DATASET_PARAMETER_COLUMN_WIDTH;
}
