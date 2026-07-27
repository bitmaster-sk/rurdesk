import { Observable } from 'rxjs';
import { Issue } from '../../issue/model/issue.model';
import { HighlightSegment } from './fuzzy.util';

export type CommandMode = 'all' | 'commands' | 'people' | 'issues' | 'navigation';

export const MODE_PREFIX: Record<Exclude<CommandMode, 'all'>, string> = {
    commands: '>',
    people: '@',
    issues: '#',
    navigation: '/'
};

/** Translator handed to pure builders so they emit ALREADY-translated display strings
 *  (titles must stay fuzzy-searchable/highlightable — raw keys would break matching). */
export type Translator = (key: string, params?: Record<string, unknown>) => string;

export interface Command {
    id: string;
    title: string;
    subtitle?: string;
    group: string;
    icon: string; // Tabler icon name (kebab), e.g. 'circle-check'
    keywords?: string;
    modes: CommandMode[]; // prefixed modes that surface this command ('all' shown everywhere)
    completion?: string; // text ⇥ fills into the input (defaults to `title`); e.g. '#428' for a jump
    run: () => void;
}

export interface RankedCommand extends Command {
    score: number;
    highlight: HighlightSegment[];
}

export interface CommandGroup {
    heading: string;
    items: RankedCommand[];
}

export interface CommandContext {
    idProject: number | null;
    issue: Issue | null; // highlighted/open issue for contextual `>` actions
}

export interface CommandProvider {
    /** Optional data load when the palette opens (e.g. fetch issues for `#`). */
    prime?(ctx: CommandContext): Observable<unknown>;
    /** Synchronous snapshot of commands offered right now. */
    getCommands(ctx: CommandContext): Command[];
    /** Optional synthetic "create" command shown when a query yields no results. */
    createFromQuery?(query: string, ctx: CommandContext): Command | null;
}

/** Result of resolving a global keydown, consumed by HotkeyService. */
export type HotkeyAction =
    | { type: 'open'; mode: CommandMode }
    | { type: 'close' }
    | { type: 'help' }
    | { type: 'list-move'; delta: 1 | -1 }
    | { type: 'none' };
