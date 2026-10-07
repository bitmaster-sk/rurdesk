import { WikiCalloutKind } from '../constants/wiki-callout-kind.enum';
import { WikiFormat } from '../constants/wiki-format.enum';
import { WikiTextEdit } from '../entity/wiki-text-edit.entity';
import { WikiFormatConverter } from './wiki-format.converter';

function apply(
    doc: string,
    from: number,
    to: number,
    format: WikiFormat
): { text: string; selected: string; cursor: number } {
    return applyEdit(doc, WikiFormatConverter.toEdit(doc, from, to, format));
}

function applyEdit(
    doc: string,
    edit: WikiTextEdit
): { text: string; selected: string; cursor: number } {
    const text = doc.slice(0, edit.from) + edit.insert + doc.slice(edit.to);
    return {
        text,
        selected: text.slice(edit.selectionFrom, edit.selectionTo),
        cursor: edit.selectionFrom
    };
}

describe('WikiFormatConverter', () => {
    it('wraps the selection in bold and keeps the word selected', () => {
        const result = apply('make it loud', 8, 12, WikiFormat.Bold);
        expect(result.text).toBe('make it **loud**');
        expect(result.selected).toBe('loud');
    });

    it('removes bold again when the selection is already wrapped', () => {
        expect(apply('make it **loud**', 10, 14, WikiFormat.Bold).text).toBe('make it loud');
        expect(apply('make it **loud**', 8, 16, WikiFormat.Bold).text).toBe('make it loud');
    });

    it('puts the cursor between markers when nothing is selected', () => {
        const result = apply('a  b', 2, 2, WikiFormat.Code);
        expect(result.text).toBe('a `` b');
        expect(result.cursor).toBe(3);
    });

    it('turns the selection into a link and selects the url to type over', () => {
        const result = apply('see docs', 4, 8, WikiFormat.Link);
        expect(result.text).toBe('see [docs](url)');
        expect(result.selected).toBe('url');
    });

    it('wraps a wiki link and leaves the cursor inside for completion', () => {
        const result = apply('see ', 4, 4, WikiFormat.WikiLink);
        expect(result.text).toBe('see [[]]');
        expect(result.cursor).toBe(6);
    });

    it('completes a link without doubling the closing brackets the toolbar already added', () => {
        const toolbar = applyEdit(
            'see [[]] now',
            WikiFormatConverter.toLinkCompletionEdit('see [[]] now', 6, 6, 'Deploy')
        );
        expect(toolbar.text).toBe('see [[Deploy]] now');
        expect(toolbar.cursor).toBe(14);
        const typed = applyEdit(
            'see [[Dep',
            WikiFormatConverter.toLinkCompletionEdit('see [[Dep', 6, 9, 'Deploy')
        );
        expect(typed.text).toBe('see [[Deploy]]');
        expect(typed.cursor).toBe(14);
    });

    it('makes the current line a heading, switches level and removes it on repeat', () => {
        expect(apply('intro\nSetup', 8, 8, WikiFormat.Heading2).text).toBe('intro\n## Setup');
        expect(apply('## Setup', 4, 4, WikiFormat.Heading3).text).toBe('### Setup');
        expect(apply('## Setup', 4, 4, WikiFormat.Heading2).text).toBe('Setup');
        expect(apply('## Setup', 4, 4, WikiFormat.Heading1).text).toBe('# Setup');
    });

    it('numbers every selected line and skips blank ones', () => {
        const doc = 'one\n\ntwo\nthree';
        expect(apply(doc, 0, doc.length, WikiFormat.OrderedList).text).toBe(
            '1. one\n\n2. two\n3. three'
        );
    });

    it('switches a bullet list to a task list and back off', () => {
        const doc = '- a\n- b';
        const tasks = apply(doc, 0, doc.length, WikiFormat.TaskList).text;
        expect(tasks).toBe('- [ ] a\n- [ ] b');
        expect(apply(tasks, 0, tasks.length, WikiFormat.TaskList).text).toBe('a\nb');
    });

    it('keeps indentation and leaves the cursor after the new prefix on an empty line', () => {
        expect(apply('  item', 3, 3, WikiFormat.BulletList).text).toBe('  - item');
        const result = apply('text\n', 5, 5, WikiFormat.Quote);
        expect(result.text).toBe('text\n> ');
        expect(result.cursor).toBe(7);
    });

    it('puts a code block on its own lines around the selection', () => {
        const result = apply('run npm ci now', 4, 10, WikiFormat.CodeBlock);
        expect(result.text).toBe('run \n```\nnpm ci\n```\n now');
        expect(result.selected).toBe('npm ci');
    });

    it('inserts a table of the picked size after a blank line with the first header selected', () => {
        const result = applyEdit(
            'Intro',
            WikiFormatConverter.toTableEdit('Intro', 5, 5, { rows: 3, columns: 2 })
        );
        expect(result.text).toBe(
            'Intro\n\n| Column 1 | Column 2 |\n| --- | --- |\n|   |   |\n|   |   |'
        );
        expect(result.selected).toBe('Column 1');
    });

    it('turns the current lines into a callout, or starts an empty one', () => {
        const doc = 'Intro\n\nRun migrations\nthen deploy';
        const edit = WikiFormatConverter.toCalloutEdit(doc, 8, doc.length, WikiCalloutKind.Warning);
        expect(applyEdit(doc, edit).text).toBe(
            'Intro\n\n> [!WARNING]\n> Run migrations\n> then deploy'
        );
        const empty = applyEdit(
            '',
            WikiFormatConverter.toCalloutEdit('', 0, 0, WikiCalloutKind.Note)
        );
        expect(empty.text).toBe('> [!NOTE]\n> ');
        expect(empty.cursor).toBe(empty.text.length);
    });
});
