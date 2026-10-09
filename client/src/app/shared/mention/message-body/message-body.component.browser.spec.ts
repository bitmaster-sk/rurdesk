import { Component, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MarkdownModule } from 'ngx-markdown';
import { MARKDOWN_MARKED_OPTIONS } from 'src/app/shared/markdown/marked-options';
import { MARKDOWN_MENTION_EXTENSION } from 'src/app/shared/markdown/mention-extension';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from 'src/app/ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { MessageBodyComponent } from './message-body.component';
import { MockupCardComponent } from 'src/app/shared/components/mockup-card/mockup-card.component';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { User } from 'src/app/auth/model/user.model';

@Component({ selector: 'app-diff-viewer', template: '', standalone: true })
class DiffViewerStub {
    public readonly rawPatch = input<string>('');
}

@Component({
    selector: 'app-mermaid-diagram',
    template: '<div data-testid="mermaid-stub">{{ source() }}</div>',
    standalone: true
})
class MermaidDiagramStub {
    public readonly source = input<string>('');
}

async function setup() {
    await TestBed.configureTestingModule({
        imports: [
            MarkdownModule.forRoot({
                markedOptions: MARKDOWN_MARKED_OPTIONS,
                markedExtensions: [MARKDOWN_MENTION_EXTENSION]
            }),
            UiModule,
            TranslateModule.forRoot(),
            TablerIconStub,
            DiffViewerStub,
            MermaidDiagramStub
        ],
        declarations: [MessageBodyComponent, MockupCardComponent]
    }).compileComponents();
}

describe('MessageBodyComponent (browser)', () => {
    beforeEach(async () => {
        await setup();
    });

    it('renders a mention chip and surrounding text for a plain comment body', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        const candidates = new Map<number, User>([
            [1, { idUser: 1, name: 'Jan', email: '', colorAvatarBg: '' }]
        ]);
        fixture.componentRef.setInput('body', 'cc @[Jan](user:1) please');
        fixture.componentRef.setInput('candidates', candidates);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const text = fixture.nativeElement.textContent as string;
        expect(text).toContain('@Jan');
        expect(text).toContain('please');

        const chips = fixture.nativeElement.querySelectorAll('.mention-chip');
        expect(chips.length).toBe(1);
    });

    it('routes a Design message with a ```diff block to app-diff-viewer (not mention-parsed)', () => {
        const diffBody =
            'Here is the plan:\n```diff\n--- a/foo.ts\n+++ b/foo.ts\n@@ -1 +1 @@\n-old\n+new\n```';
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', diffBody);
        fixture.componentRef.setInput('messageKind', MessageKind.Design);
        fixture.detectChanges();

        const diffViewers = fixture.nativeElement.querySelectorAll('app-diff-viewer');
        expect(diffViewers.length).toBe(1);

        // Diff content should not be mangled by mention parsing (no spurious chips).
        const chips = fixture.nativeElement.querySelectorAll('.mention-chip');
        expect(chips.length).toBe(0);
    });

    it('routes a Design message with a ```mermaid block to app-mermaid-diagram', async () => {
        const body = 'Diagram:\n```mermaid\nflowchart TD\n    A --> B\n```';
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', body);
        fixture.componentRef.setInput('messageKind', MessageKind.Design);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const diagrams = fixture.nativeElement.querySelectorAll('app-mermaid-diagram');
        expect(diagrams.length).toBe(1);
        const stub = fixture.nativeElement.querySelector('[data-testid="mermaid-stub"]');
        expect(stub.textContent).toContain('flowchart TD');
        expect(fixture.nativeElement.textContent).toContain('Diagram:');
    });

    it('renders a ```mermaid block in a user comment as a diagram, not a code block', async () => {
        const body = 'Here is the flow:\n```mermaid\nflowchart LR\n    a --> b\n```\ndone.';
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', body);
        fixture.componentRef.setInput('messageKind', MessageKind.Comment);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelectorAll('app-mermaid-diagram').length).toBe(1);
        const stub = fixture.nativeElement.querySelector('[data-testid="mermaid-stub"]');
        expect(stub.textContent).toContain('flowchart LR');
    });

    it('keeps a ```diff block in a user comment as plain text, not a diff viewer', async () => {
        const body = '```diff\n--- a/f\n+++ b/f\n@@ @@\n+x\n```';
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', body);
        fixture.componentRef.setInput('messageKind', MessageKind.Comment);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelectorAll('app-diff-viewer').length).toBe(0);
        expect(fixture.nativeElement.textContent).toContain('+x');
    });

    it('renders a mention token inside a single-backtick inline span literally (no chip) in a non-agent body', async () => {
        // Inline code span wrapping the mention token — splitCodeSpans must treat it as code.
        const body = '`@[x](user:1)`';
        const fixture = TestBed.createComponent(MessageBodyComponent);
        const candidates = new Map<number, User>([
            [1, { idUser: 1, name: 'x', email: '', colorAvatarBg: '' }]
        ]);
        fixture.componentRef.setInput('body', body);
        fixture.componentRef.setInput('candidates', candidates);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        // The token is inside inline code — no chip must be rendered.
        const chips = fixture.nativeElement.querySelectorAll('.mention-chip');
        expect(chips.length).toBe(0);
    });

    it('renders a mention token inside a fenced code block literally (no chip) in a non-agent body', () => {
        // The mention token is INSIDE a triple-backtick code fence — must stay literal.
        const body = 'Look at this:\n```\n@[x](user:1)\n```\nend';
        const fixture = TestBed.createComponent(MessageBodyComponent);
        const candidates = new Map<number, User>([
            [1, { idUser: 1, name: 'x', email: '', colorAvatarBg: '' }]
        ]);
        fixture.componentRef.setInput('body', body);
        fixture.componentRef.setInput('candidates', candidates);
        // No messageKind set — defaults to undefined (user-typed message).
        fixture.detectChanges();

        // No chip should be rendered for the token inside the fence.
        const chips = fixture.nativeElement.querySelectorAll('.mention-chip');
        expect(chips.length).toBe(0);
    });

    it('mixed text+mention renders inline: chip and trailing text share the same visual line', async () => {
        // "cc @[Jan](user:1) please" must NOT stack vertically:
        // the mention chip and the trailing " please" text run must share
        // the same top offset (i.e. they are on the same line).
        const fixture = TestBed.createComponent(MessageBodyComponent);
        const candidates = new Map<number, User>([
            [1, { idUser: 1, name: 'Jan', email: '', colorAvatarBg: '' }]
        ]);
        fixture.componentRef.setInput('body', 'cc @[Jan](user:1) please');
        fixture.componentRef.setInput('candidates', candidates);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const host: HTMLElement = fixture.nativeElement;

        // The mention chip element.
        const chip = host.querySelector('.mention-chip') as HTMLElement;
        expect(chip).not.toBeNull();

        // The text run element (.text-run) — now a single run containing the chip inline.
        const textRuns = host.querySelectorAll<HTMLElement>('.text-run');
        // With the unified markdown renderer there is one text run.
        expect(textRuns.length).toBeGreaterThanOrEqual(1);
        const run = textRuns[0];

        // The chip must be inside the text run (inline, not a separate block).
        expect(run.contains(chip)).toBe(true);

        // The text run must be rendered inline (not block) so the chip sits on the same line.
        const display = window.getComputedStyle(run).display;
        expect(display).not.toBe('block');
    });

    it('renders a single newline as a line break, not as a space', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', 'first line\nsecond line');
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const run = fixture.nativeElement.querySelector('.text-run') as HTMLElement;
        expect(run.querySelectorAll('br').length).toBe(1);
        expect(run.querySelectorAll('p').length).toBe(1);
    });

    it('still starts a new paragraph on a blank line', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', 'first paragraph\n\nsecond paragraph');
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const run = fixture.nativeElement.querySelector('.text-run') as HTMLElement;
        expect(run.querySelectorAll('p').length).toBe(2);
    });

    it('keeps a fenced code block verbatim — no line breaks injected inside it', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', '```\nline one\nline two\n```');
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const code = fixture.nativeElement.querySelector('pre code') as HTMLElement;
        expect(code.querySelectorAll('br').length).toBe(0);
        expect(code.textContent).toContain('line one\nline two');
    });

    it('renders inline backticks as a code element the stylesheet can target', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', 'see `issue_repository.go` for details');
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const code = fixture.nativeElement.querySelector('code') as HTMLElement;
        expect(code).not.toBeNull();
        expect(code.textContent).toBe('issue_repository.go');
        expect(code.closest('pre')).toBeNull();
    });

    it('keeps a table column alignment attribute the stylesheet can target', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', '| A | B |\n| --- | ---: |\n| 1 | 2 |');
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const host: HTMLElement = fixture.nativeElement;
        expect(host.querySelectorAll('table thead th').length).toBe(2);
        expect(host.querySelector('td[align="right"]')).not.toBeNull();
    });

    it('emits the mockup ref when an approvable mockup card requests approval', () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', '```mockup title="B"\n<p>b</p>\n```\n');
        fixture.componentRef.setInput('messageKind', MessageKind.Design);
        fixture.componentRef.setInput('approvable', true);
        let ref = '';
        fixture.componentInstance.useMockup.subscribe(r => (ref = r));
        fixture.detectChanges();

        const btn = fixture.nativeElement.querySelector('.mockup-card__approve') as HTMLElement;
        expect(btn).toBeTruthy();
        btn.click();
        expect(ref).toBe('B #1');
    });

    it('marks the approved mockup selected and the others rejected via selectedMockupRef', () => {
        const body = '```mockup title="A"\n<p>a</p>\n```\n```mockup title="B"\n<p>b</p>\n```\n';
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', body);
        fixture.componentRef.setInput('messageKind', MessageKind.Design);
        fixture.componentRef.setInput('selectedMockupRef', 'B #2');
        fixture.detectChanges();

        expect(fixture.nativeElement.querySelectorAll('.mockup-card--selected').length).toBe(1);
        expect(fixture.nativeElement.querySelectorAll('.mockup-card--rejected').length).toBe(1);
    });

    it('renders code-corrupting patterns verbatim — no character substitution', async () => {
        // Patterns that the old pipe corrupted: one-letter ascii emoticon
        // patterns (:b, :p, :o) inside object literals, type annotations, and URLs.
        const body = [
            'See this code:',
            '```ts',
            'const x = {a:b};',
            'let p:Promise<void>',
            '```',
            'And inline: {name:props.name} and https://x.com/a:b'
        ].join('\n');
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', body);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const text = fixture.nativeElement.textContent as string;
        // The code block content must be preserved character-for-character.
        expect(text).toContain('const x = {a:b};');
        expect(text).toContain('let p:Promise<void>');
        // Inline text must also be preserved.
        expect(text).toContain('{name:props.name}');
        expect(text).toContain('https://x.com/a:b');
        // No pictographic characters should have been injected.
        expect(text).not.toContain('😜');
        expect(text).not.toContain('😄');
        expect(text).not.toContain('😲');
    });

    it('renders a table with inline code in a cell as one <table> with <code>', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', '| a | `x` |\n| --- | --- |\n| 1 | 2 |');
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const host: HTMLElement = fixture.nativeElement;
        expect(host.querySelectorAll('table').length).toBe(1);
        const code = host.querySelector('table code') as HTMLElement;
        expect(code).not.toBeNull();
        expect(code.textContent).toBe('x');
    });

    it('renders a list with inline code in two items as one <ul> with two <li>', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', '- item `a`\n- item `b`');
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const host: HTMLElement = fixture.nativeElement;
        expect(host.querySelectorAll('ul').length).toBe(1);
        expect(host.querySelectorAll('ul li').length).toBe(2);
    });

    it('renders a mention in a table cell as a .mention-chip inside a <td>', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        const candidates = new Map<number, User>([
            [1, { idUser: 1, name: 'Jan', email: '', colorAvatarBg: '' }]
        ]);
        fixture.componentRef.setInput('body', '| user |\n| --- |\n| @[Jan](user:1) |');
        fixture.componentRef.setInput('candidates', candidates);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const host: HTMLElement = fixture.nativeElement;
        expect(host.querySelectorAll('table').length).toBe(1);
        const chip = host.querySelector('table td .mention-chip') as HTMLElement;
        expect(chip).not.toBeNull();
        expect(chip.textContent).toBe('@Jan');
    });

    it('renders a mention in a list item as a .mention-chip inside an <li>', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        const candidates = new Map<number, User>([
            [1, { idUser: 1, name: 'Jan', email: '', colorAvatarBg: '' }]
        ]);
        fixture.componentRef.setInput('body', '- cc @[Jan](user:1)');
        fixture.componentRef.setInput('candidates', candidates);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const host: HTMLElement = fixture.nativeElement;
        expect(host.querySelectorAll('ul').length).toBe(1);
        const chip = host.querySelector('ul li .mention-chip') as HTMLElement;
        expect(chip).not.toBeNull();
        expect(chip.textContent).toBe('@Jan');
    });

    it('resolves live name from candidates in a mention token', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        const candidates = new Map<number, User>([
            [1, { idUser: 1, name: 'Nový', email: '', colorAvatarBg: '' }]
        ]);
        fixture.componentRef.setInput('body', 'cc @[Starý](user:1) please');
        fixture.componentRef.setInput('candidates', candidates);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const chip = fixture.nativeElement.querySelector('.mention-chip') as HTMLElement;
        expect(chip).not.toBeNull();
        expect(chip.textContent).toBe('@Nový');
    });

    it('shows stored name when candidate is absent', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        fixture.componentRef.setInput('body', 'cc @[Starý](user:1) please');
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const chip = fixture.nativeElement.querySelector('.mention-chip') as HTMLElement;
        expect(chip).not.toBeNull();
        expect(chip.textContent).toBe('@Starý');
    });

    it('escapes HTML in mention names so <b> renders as text not an element', async () => {
        const fixture = TestBed.createComponent(MessageBodyComponent);
        const candidates = new Map<number, User>([
            [1, { idUser: 1, name: '<b>x</b>', email: '', colorAvatarBg: '' }]
        ]);
        fixture.componentRef.setInput('body', 'cc @[x](user:1)');
        fixture.componentRef.setInput('candidates', candidates);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();

        const host: HTMLElement = fixture.nativeElement;
        expect(host.querySelector('.mention-chip b')).toBeNull();
        const chip = host.querySelector('.mention-chip') as HTMLElement;
        expect(chip.textContent).toBe('@<b>x</b>');
    });
});
