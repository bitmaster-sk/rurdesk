import { Command, CommandGroup, CommandMode, RankedCommand } from './command.model';
import { fuzzyMatch, highlight } from './fuzzy.util';

export function buildGroups(
    commands: Command[],
    mode: CommandMode,
    query: string,
    recentIds: string[]
): CommandGroup[] {
    const ranked: RankedCommand[] = [];
    for (const command of commands) {
        if (mode !== 'all' && !command.modes.includes(mode)) continue;
        const haystack = [command.title, command.subtitle, command.keywords]
            .filter(Boolean)
            .join(' ');
        const { matched, score } = fuzzyMatch(query, haystack);
        if (!matched) continue;
        ranked.push({ ...command, score, highlight: highlight(command.title, query) });
    }

    const norm = query.trim().toLowerCase();
    const recentRank = (id: string): number => {
        const idx = recentIds.indexOf(id);
        return idx < 0 ? Number.POSITIVE_INFINITY : idx;
    };
    // Exact-token match: whole query equals one whitespace-token of the haystack (case-insensitive).
    // Makes `428` win for a `#428 …` jump without buildGroups knowing about IDs.
    const isExact = (c: RankedCommand): boolean =>
        !!norm &&
        [c.title, c.subtitle, c.keywords]
            .filter(Boolean)
            .join(' ')
            .toLowerCase()
            .split(/\s+/)
            .includes(norm);
    ranked.sort((a, b) => {
        if (norm) {
            // Non-empty query: exact-token first → score → recency tiebreak → alpha.
            const ea = isExact(a) ? 0 : 1;
            const eb = isExact(b) ? 0 : 1;
            if (ea !== eb) return ea - eb;
            if (b.score !== a.score) return b.score - a.score;
            const ra = recentRank(a.id),
                rb = recentRank(b.id);
            if (ra !== rb) return ra - rb;
            return a.title.localeCompare(b.title);
        }
        // Empty/browse query: recency-first (idle/browse state only), then score, then alpha.
        const ra = recentRank(a.id),
            rb = recentRank(b.id);
        if (ra !== rb) return ra - rb;
        if (b.score !== a.score) return b.score - a.score;
        return a.title.localeCompare(b.title);
    });

    const groups: CommandGroup[] = [];
    const byHeading = new Map<string, CommandGroup>();
    for (const item of ranked) {
        let group = byHeading.get(item.group);
        if (!group) {
            group = { heading: item.group, items: [] };
            byHeading.set(item.group, group);
            groups.push(group);
        }
        group.items.push(item);
    }
    return groups;
}
