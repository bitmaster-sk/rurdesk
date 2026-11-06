import { HORIZONTAL_OFFSET } from '../../../service/gantt-arrow-routing';

export const BAR_GAP = 12;
export const SUMMARY_BAR_HEIGHT = 12;
// Must exceed HORIZONTAL_OFFSET so the label clears the arrow stub extending past the bar edge
export const OVERFLOW_LABEL_GAP = HORIZONTAL_OFFSET + 4;
