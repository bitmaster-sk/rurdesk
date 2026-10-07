import { WikiCalloutKind } from '../constants/wiki-callout-kind.enum';
import { WikiFormat } from '../constants/wiki-format.enum';
import { WikiTableSize } from '../entity/wiki-table-size.entity';
import { WikiTextEdit } from '../entity/wiki-text-edit.entity';

interface WikiLineFormat {
    pattern: RegExp;
    prefix: (index: number) => string;
    strip: RegExp;
}

export abstract class WikiFormatConverter {
    private static readonly listMarker = /^[-*+] (\[[ xX]\] )?|^\d+[.)] /;

    private static readonly lineFormats: Partial<Record<WikiFormat, WikiLineFormat>> = {
        [WikiFormat.Heading1]: { pattern: /^# /, prefix: () => '# ', strip: /^#{1,6} / },
        [WikiFormat.Heading2]: { pattern: /^## (?!#)/, prefix: () => '## ', strip: /^#{1,6} / },
        [WikiFormat.Heading3]: { pattern: /^### (?!#)/, prefix: () => '### ', strip: /^#{1,6} / },
        [WikiFormat.BulletList]: {
            pattern: /^[-*+] (?!\[[ xX]\] )/,
            prefix: () => '- ',
            strip: WikiFormatConverter.listMarker
        },
        [WikiFormat.TaskList]: {
            pattern: /^[-*+] \[[ xX]\] /,
            prefix: () => '- [ ] ',
            strip: WikiFormatConverter.listMarker
        },
        [WikiFormat.OrderedList]: {
            pattern: /^\d+[.)] /,
            prefix: index => `${index + 1}. `,
            strip: WikiFormatConverter.listMarker
        },
        [WikiFormat.Quote]: { pattern: /^> ?/, prefix: () => '> ', strip: /^> ?/ }
    };

    private static readonly inlineMarkers: Partial<Record<WikiFormat, string>> = {
        [WikiFormat.Bold]: '**',
        [WikiFormat.Italic]: '_',
        [WikiFormat.Strikethrough]: '~~',
        [WikiFormat.Code]: '`'
    };

    public static toEdit(doc: string, from: number, to: number, format: WikiFormat): WikiTextEdit {
        const marker = WikiFormatConverter.inlineMarkers[format];
        if (marker) {
            return WikiFormatConverter.toInline(doc, from, to, marker);
        }
        const lineFormat = WikiFormatConverter.lineFormats[format];
        if (lineFormat) {
            return WikiFormatConverter.toLines(doc, from, to, lineFormat);
        }
        switch (format) {
            case WikiFormat.Link:
                return WikiFormatConverter.toLink(doc, from, to);
            case WikiFormat.WikiLink:
                return WikiFormatConverter.toWrapped(doc, from, to, '[[', ']]');
            default:
                return WikiFormatConverter.toCodeBlock(doc, from, to);
        }
    }

    public static toLinkCompletionEdit(
        doc: string,
        from: number,
        to: number,
        target: string
    ): WikiTextEdit {
        const isClosed = doc.slice(to, to + 2) === ']]';
        const insert = isClosed ? target : `${target}]]`;
        const cursor = from + target.length + 2;
        return { from, to, insert, selectionFrom: cursor, selectionTo: cursor };
    }

    public static toTableEdit(
        doc: string,
        from: number,
        to: number,
        size: WikiTableSize
    ): WikiTextEdit {
        const columns = Math.max(1, size.columns);
        const header = Array.from({ length: columns }, (_, index) => `Column ${index + 1}`);
        const row = (cells: string[]): string => `| ${cells.join(' | ')} |`;
        const lines = [
            row(header),
            row(header.map(() => '---')),
            ...Array.from({ length: Math.max(0, size.rows - 1) }, () => row(header.map(() => ' ')))
        ];
        const before = WikiFormatConverter.toBlockGap(doc, from);
        const after = to === doc.length || doc[to] === '\n' ? '' : '\n';
        const headerFrom = from + before.length + 2;
        return {
            from,
            to,
            insert: `${before}${lines.join('\n')}${after}`,
            selectionFrom: headerFrom,
            selectionTo: headerFrom + header[0].length
        };
    }

    public static toCalloutEdit(
        doc: string,
        from: number,
        to: number,
        kind: WikiCalloutKind
    ): WikiTextEdit {
        const { lineFrom, lineTo } = WikiFormatConverter.toLineRange(doc, from, to);
        const lines = doc.slice(lineFrom, lineTo).split('\n');
        const isEmpty = lines.length === 1 && lines[0] === '';
        const body = isEmpty ? ['> '] : lines.map(line => (line === '' ? '>' : `> ${line}`));
        const before = WikiFormatConverter.toBlockGap(doc, lineFrom);
        const insert = `${before}> [!${kind.toUpperCase()}]\n${body.join('\n')}`;
        const cursor = lineFrom + insert.length;
        return { from: lineFrom, to: lineTo, insert, selectionFrom: cursor, selectionTo: cursor };
    }

    private static toInline(doc: string, from: number, to: number, marker: string): WikiTextEdit {
        const size = marker.length;
        const selected = doc.slice(from, to);
        if (doc.slice(from - size, from) === marker && doc.slice(to, to + size) === marker) {
            return {
                from: from - size,
                to: to + size,
                insert: selected,
                selectionFrom: from - size,
                selectionTo: to - size
            };
        }
        if (
            selected.length >= size * 2 &&
            selected.startsWith(marker) &&
            selected.endsWith(marker)
        ) {
            const inner = selected.slice(size, -size);
            return {
                from,
                to,
                insert: inner,
                selectionFrom: from,
                selectionTo: from + inner.length
            };
        }
        return WikiFormatConverter.toWrapped(doc, from, to, marker, marker);
    }

    private static toWrapped(
        doc: string,
        from: number,
        to: number,
        open: string,
        close: string
    ): WikiTextEdit {
        const selected = doc.slice(from, to);
        return {
            from,
            to,
            insert: `${open}${selected}${close}`,
            selectionFrom: from + open.length,
            selectionTo: from + open.length + selected.length
        };
    }

    private static toLink(doc: string, from: number, to: number): WikiTextEdit {
        const selected = doc.slice(from, to);
        if (selected.length === 0) {
            return { from, to, insert: '[](url)', selectionFrom: from + 1, selectionTo: from + 1 };
        }
        const urlFrom = from + selected.length + 3;
        return {
            from,
            to,
            insert: `[${selected}](url)`,
            selectionFrom: urlFrom,
            selectionTo: urlFrom + 3
        };
    }

    private static toLines(
        doc: string,
        from: number,
        to: number,
        format: WikiLineFormat
    ): WikiTextEdit {
        const { lineFrom, lineTo } = WikiFormatConverter.toLineRange(doc, from, to);
        const lines = doc.slice(lineFrom, lineTo).split('\n');
        const hasText = lines.some(line => line.trim() !== '');
        const targets = lines.map(line => !hasText || line.trim() !== '');
        const isApplied = lines.every(
            (line, index) => !targets[index] || format.pattern.test(line.trimStart())
        );
        let counter = 0;
        const formatted = lines.map((line, index) => {
            if (!targets[index]) {
                return line;
            }
            const indent = line.slice(0, line.length - line.trimStart().length);
            const content = line.trimStart();
            if (isApplied) {
                return indent + content.replace(format.pattern, '');
            }
            return indent + format.prefix(counter++) + content.replace(format.strip, '');
        });
        const insert = formatted.join('\n');
        if (from === to && lines.length === 1) {
            const cursor = Math.max(lineFrom, from + insert.length - lines[0].length);
            return {
                from: lineFrom,
                to: lineTo,
                insert,
                selectionFrom: cursor,
                selectionTo: cursor
            };
        }
        return {
            from: lineFrom,
            to: lineTo,
            insert,
            selectionFrom: lineFrom,
            selectionTo: lineFrom + insert.length
        };
    }

    private static toLineRange(
        doc: string,
        from: number,
        to: number
    ): { lineFrom: number; lineTo: number } {
        const end = to > from && doc[to - 1] === '\n' ? to - 1 : to;
        const newline = doc.indexOf('\n', end);
        return {
            lineFrom: doc.lastIndexOf('\n', from - 1) + 1,
            lineTo: newline === -1 ? doc.length : newline
        };
    }

    private static toCodeBlock(doc: string, from: number, to: number): WikiTextEdit {
        const selected = doc.slice(from, to).replace(/\n$/, '');
        const before = WikiFormatConverter.isLineStart(doc, from) ? '' : '\n';
        const after = to === doc.length || doc[to] === '\n' ? '' : '\n';
        const open = `${before}\`\`\`\n`;
        return {
            from,
            to,
            insert: `${open}${selected}\n\`\`\`${after}`,
            selectionFrom: from + open.length,
            selectionTo: from + open.length + selected.length
        };
    }

    private static toBlockGap(doc: string, position: number): string {
        if (position === 0 || doc.slice(position - 2, position) === '\n\n') {
            return '';
        }
        return WikiFormatConverter.isLineStart(doc, position) ? '\n' : '\n\n';
    }

    private static isLineStart(doc: string, position: number): boolean {
        return position === 0 || doc[position - 1] === '\n';
    }
}
