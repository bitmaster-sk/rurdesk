import { CommandMode, MODE_PREFIX } from './command.model';

const PREFIX_TO_MODE: Record<string, CommandMode> = Object.fromEntries(
    Object.entries(MODE_PREFIX).map(([mode, prefix]) => [prefix, mode as CommandMode])
);

export function parseQuery(raw: string): { mode: CommandMode; query: string } {
    const first = raw[0];
    const mode = first ? PREFIX_TO_MODE[first] : undefined;
    return mode ? { mode, query: raw.slice(1).trim() } : { mode: 'all', query: raw.trim() };
}
