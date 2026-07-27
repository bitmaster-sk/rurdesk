import { Component, Pipe, PipeTransform, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MarkdownModule } from 'ngx-markdown';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from 'src/app/ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';
import { MessageBodyComponent } from './message-body.component';
import { MockupCardComponent } from 'src/app/shared/components/mockup-card/mockup-card.component';
import { MentionChipComponent } from 'src/app/shared/mention/mention-chip/mention-chip.component';
import { MessageKind } from 'src/app/message/constant/message-kind.enum';
import { User } from 'src/app/auth/model/user.model';

@Pipe({ name: 'emoji', standalone: false })
class StubEmojiPipe implements PipeTransform {
    public transform(v: string): string {
        return v;
    }
}

@Component({ selector: 'app-diff-viewer', template: '', standalone: true })
class DiffViewerStub {
    public readonly rawPatch = input<string>('');
}

async function setup() {
    await TestBed.configureTestingModule({
        imports: [
            MarkdownModule.forRoot(),
            UiModule,
            TranslateModule.forRoot(),
            TablerIconStub,
            DiffViewerStub
        ],
        declarations: [
            MessageBodyComponent,
            MockupCardComponent,
            MentionChipComponent,
            StubEmojiPipe
        ]
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

        const chips = fixture.nativeElement.querySelectorAll('app-mention-chip');
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
        const chips = fixture.nativeElement.querySelectorAll('app-mention-chip');
        expect(chips.length).toBe(0);
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
        const chips = fixture.nativeElement.querySelectorAll('app-mention-chip');
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
        const chips = fixture.nativeElement.querySelectorAll('app-mention-chip');
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
        const chip = host.querySelector('app-mention-chip') as HTMLElement;
        expect(chip).not.toBeNull();

        // The text run element (.text-run) that follows the chip (" please").
        const textRuns = host.querySelectorAll<HTMLElement>('.text-run');
        // At minimum there must be two text runs ("cc " before, " please" after).
        expect(textRuns.length).toBeGreaterThanOrEqual(2);
        const trailingRun = textRuns[textRuns.length - 1];

        // Both must be rendered inline (not block) so they sit on the same line.
        const chipRect = chip.getBoundingClientRect();
        const textRect = trailingRun.getBoundingClientRect();

        // Their vertical centres must overlap (within 4 px of each other).
        const chipMid = chipRect.top + chipRect.height / 2;
        const textMid = textRect.top + textRect.height / 2;
        expect(Math.abs(chipMid - textMid)).toBeLessThan(4);

        // Also verify the text run's computed display is not 'block'.
        const display = window.getComputedStyle(trailingRun).display;
        expect(display).not.toBe('block');
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
});
