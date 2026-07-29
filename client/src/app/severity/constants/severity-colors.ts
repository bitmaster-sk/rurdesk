/**
 * Palette offered to a new severity. Same weight as the seeded Low/Medium/High
 */
export const SEVERITY_COLORS: readonly string[] = [
    '#e64a19',
    '#f57c00',
    '#689f38',
    '#388e3c',
    '#00897b',
    '#0097a7',
    '#1976d2',
    '#3f51b5',
    '#7b1fa2',
    '#c2185b',
    '#5d4037',
    '#546e7a'
];

export function randomSeverityColor(): string {
    return SEVERITY_COLORS[Math.floor(Math.random() * SEVERITY_COLORS.length)];
}
