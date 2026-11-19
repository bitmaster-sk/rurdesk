import { TaskDraft } from './task-draft.entity';

export const TASK_DRAFT_TITLE_MAX = 100;

const TITLE_PREFIX = '# ';

export abstract class TaskDraftConverter {
    /**
     * The agent writes the draft as Markdown whose first `# ` line is the title;
     * the rest is the description. A draft without one is all description.
     */
    public static toDraft(body: string): TaskDraft {
        const lines = body.split('\n');
        const index = lines.findIndex(line => line.startsWith(TITLE_PREFIX));
        if (index < 0) {
            return { title: '', description: body.trim() };
        }
        return {
            title: lines[index].slice(TITLE_PREFIX.length).trim().slice(0, TASK_DRAFT_TITLE_MAX),
            description: lines
                .filter((_, i) => i !== index)
                .join('\n')
                .trim()
        };
    }

    /** The saved description ends with a line linking back to the conversation it came from. */
    public static toDescription(description: string, linkLine: string): string {
        return `${description.trimEnd()}\n\n---\n${linkLine}`;
    }
}
