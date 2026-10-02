import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { SharedModule } from '../../shared.module';
import { DiffViewerComponent } from './diff-viewer.component';
import { Observable, Subject, of, throwError } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { MrDiff } from 'src/app/project/model/git-integration.model';

const RAW_PATCH = [
    '--- a/hello.txt',
    '+++ b/hello.txt',
    '@@ -1,2 +1,2 @@',
    ' keep',
    '-old line',
    '+new line'
].join('\n');

const FILE_LINES = Array.from({ length: 20 }, (_, idx) => `line ${idx + 1}`);

const STRUCTURED_DIFF: MrDiff = {
    headSha: 'head-sha',
    files: [
        {
            oldPath: 'hello.txt',
            newPath: 'hello.txt',
            patch: [
                '@@ -3,2 +3,2 @@',
                '-old 3',
                '+line 3',
                ' line 4',
                '@@ -15,2 +15,2 @@',
                ' line 15',
                '-old 16',
                '+line 16',
                ''
            ].join('\n')
        },
        {
            oldPath: 'new.txt',
            newPath: 'new.txt',
            patch: '@@ -0,0 +1,2 @@\n+a\n+b\n'
        }
    ]
};

const toast = { showError: vi.fn() };

describe('DiffViewerComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [SharedModule, TranslateModule.forRoot()],
            providers: [{ provide: ToastNotificationService, useValue: toast }]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(DiffViewerComponent);
        fixture.componentRef.setInput('rawPatch', RAW_PATCH);
        fixture.detectChanges();
        const header = fixture.nativeElement.querySelector('.d2h-file-header') as HTMLElement;
        const body = fixture.nativeElement.querySelector('.d2h-file-diff') as HTMLElement;
        return { fixture, header, body };
    }

    it('exposes the file header as a focusable, expanded button', () => {
        const { header } = setup();
        expect(header.getAttribute('role')).toBe('button');
        expect(header.getAttribute('tabindex')).toBe('0');
        expect(header.getAttribute('aria-expanded')).toBe('true');
    });

    it('collapses and re-expands the file on Enter', () => {
        const { header, body } = setup();

        header.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        expect(body.classList).toContain('d2h-d-none');
        expect(header.getAttribute('aria-expanded')).toBe('false');

        header.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        expect(body.classList).not.toContain('d2h-d-none');
        expect(header.getAttribute('aria-expanded')).toBe('true');
    });

    it('collapses on Space too', () => {
        const { header, body } = setup();
        header.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));
        expect(body.classList).toContain('d2h-d-none');
    });

    it('keeps aria-expanded in sync when toggled by mouse', () => {
        const { header } = setup();
        header.click();
        expect(header.getAttribute('aria-expanded')).toBe('false');
    });
});

describe('DiffViewerComponent context expansion (browser)', () => {
    beforeEach(async () => {
        toast.showError.mockReset();
        await TestBed.configureTestingModule({
            imports: [SharedModule, TranslateModule.forRoot()],
            providers: [{ provide: ToastNotificationService, useValue: toast }]
        }).compileComponents();
    });

    function setup(load: () => Observable<string[]> = () => of(FILE_LINES)) {
        const fixture = TestBed.createComponent(DiffViewerComponent);
        const loader = vi.fn(load);
        fixture.componentRef.setInput('diff', STRUCTURED_DIFF);
        fixture.componentRef.setInput('fileContentLoader', loader);
        fixture.detectChanges();
        const root = fixture.nativeElement as HTMLElement;
        const firstFile = () => root.querySelectorAll<HTMLElement>('.d2h-file-wrapper')[0];
        const buttons = () =>
            Array.from(firstFile().querySelectorAll<HTMLButtonElement>('.diff-viewer__expand'));
        const visibleLines = () =>
            Array.from(firstFile().querySelectorAll('.d2h-code-line-ctn')).map(
                el => el.textContent?.trim() ?? ''
            );
        return { root, loader, buttons, visibleLines };
    }

    it('keeps the diff2html file header and points the expand buttons up at the top, both ways between hunks and down at the end', () => {
        const { root, buttons } = setup();
        expect(root.querySelectorAll('.d2h-file-header')).toHaveLength(2);
        expect(root.querySelector('.diff-viewer__added')?.textContent).toBe('+2');
        expect(buttons().map(button => button.getAttribute('aria-label'))).toEqual([
            'DIFF.EXPAND_UP',
            'DIFF.EXPAND_ALL',
            'DIFF.EXPAND_DOWN'
        ]);
    });

    it('tags added and deleted files like diff2html does for git patches', () => {
        const fixture = TestBed.createComponent(DiffViewerComponent);
        fixture.componentRef.setInput('diff', {
            headSha: 'head-sha',
            files: [
                ...STRUCTURED_DIFF.files,
                { oldPath: 'gone.txt', newPath: 'gone.txt', patch: '@@ -1,2 +0,0 @@\n-a\n-b\n' }
            ]
        });
        fixture.detectChanges();
        const tags = Array.from(
            (fixture.nativeElement as HTMLElement).querySelectorAll('.d2h-file-wrapper .d2h-tag')
        ).map(tag => tag.textContent?.trim());
        expect(tags).toEqual(['CHANGED', 'ADDED', 'DELETED']);
    });

    it('offers no expansion for an added file', () => {
        const { root } = setup();
        const addedFile = root.querySelectorAll<HTMLElement>('.d2h-file-wrapper')[1];
        expect(addedFile.querySelectorAll('.diff-viewer__expand')).toHaveLength(0);
    });

    it('reveals the hidden lines above the first hunk from the head commit', () => {
        const { loader, buttons, visibleLines } = setup();
        buttons()[0].click();

        expect(loader).toHaveBeenCalledWith('hello.txt', 'head-sha');
        expect(visibleLines().slice(0, 3)).toEqual(['line 1', 'line 2', 'old 3']);
        expect(buttons()).toHaveLength(2);
    });

    it('merges two hunks into one once the gap between them is revealed', () => {
        const { root, buttons, visibleLines } = setup();
        buttons()[1].click();

        const firstFile = root.querySelectorAll<HTMLElement>('.d2h-file-wrapper')[0];
        const hunkHeaders = Array.from(firstFile.querySelectorAll('td.d2h-info .d2h-code-line'))
            .map(el => el.textContent?.trim() ?? '')
            .filter(text => text.startsWith('@@'));
        expect(hunkHeaders).toEqual(['@@ -3,14 +3,14 @@']);
        expect(visibleLines()).toContain('line 10');
    });

    it('fetches the file once and serves later expansions from memory', () => {
        const { loader, buttons } = setup();
        buttons()[0].click();
        buttons()[0].click();
        expect(loader).toHaveBeenCalledTimes(1);
    });

    it('reveals the lines below the last hunk and drops the button at the end of the file', () => {
        const { buttons, visibleLines } = setup();
        buttons()[2].click();

        expect(visibleLines().slice(-4)).toEqual(['line 17', 'line 18', 'line 19', 'line 20']);
        expect(buttons().map(button => button.getAttribute('aria-label'))).toEqual([
            'DIFF.EXPAND_UP',
            'DIFF.EXPAND_ALL'
        ]);
    });

    it('ignores repeated clicks while the file is still loading', () => {
        const response = new Subject<string[]>();
        const { loader, buttons, visibleLines } = setup(() => response);
        const [firstButton] = buttons();
        firstButton.click();
        firstButton.click();
        response.next(FILE_LINES);

        expect(loader).toHaveBeenCalledTimes(1);
        expect(visibleLines().filter(line => line === 'line 1')).toHaveLength(1);
    });

    it('shows a spinner on the clicked button and blocks the file while its lines load', () => {
        const response = new Subject<string[]>();
        const { buttons } = setup(() => response);
        buttons()[0].click();

        const [clicked, ...others] = buttons();
        expect(clicked.getAttribute('aria-busy')).toBe('true');
        expect(clicked.classList).toContain('diff-viewer__expand--loading');
        expect(others.every(button => button.disabled)).toBe(true);

        response.next(FILE_LINES);
        expect(buttons().some(button => button.disabled || button.hasAttribute('aria-busy'))).toBe(
            false
        );
    });

    it('re-enables the buttons after a failed load', () => {
        const { buttons } = setup(() => throwError(() => ({ status: 502 })));
        buttons()[0].click();
        expect(buttons()).toHaveLength(3);
        expect(buttons().some(button => button.disabled || button.hasAttribute('aria-busy'))).toBe(
            false
        );
    });

    it('asks to reload when the change request moved on since the diff was loaded', () => {
        const { buttons } = setup(() => throwError(() => ({ status: 409 })));
        buttons()[0].click();
        expect(toast.showError).toHaveBeenCalledWith('DIFF.EXPAND_STALE');
    });

    it('reports a failed load', () => {
        const { buttons } = setup(() => throwError(() => ({ status: 502 })));
        buttons()[0].click();
        expect(toast.showError).toHaveBeenCalledWith('DIFF.EXPAND_FAILED');
    });

    it('pairs every file with its own stats and expansion when one file has an empty patch', () => {
        const hunk = (name: string) =>
            ['@@ -5,2 +5,2 @@', `-old ${name}`, `+new ${name}`, ' line 6', ''].join('\n');
        const fixture = TestBed.createComponent(DiffViewerComponent);
        const loader = vi.fn(() => of(FILE_LINES));
        fixture.componentRef.setInput('diff', {
            headSha: 'head-sha',
            files: [
                { oldPath: 'a.txt', newPath: 'a.txt', patch: hunk('a') },
                { oldPath: 'logo.png', newPath: 'logo.png', patch: '' },
                { oldPath: 'b.txt', newPath: 'b.txt', patch: hunk('b') + '+extra b\n' },
                { oldPath: 'c.txt', newPath: 'c.txt', patch: hunk('c') }
            ]
        } satisfies MrDiff);
        fixture.componentRef.setInput('fileContentLoader', loader);
        fixture.detectChanges();

        const root = fixture.nativeElement as HTMLElement;
        const wrapperOf = (name: string) =>
            Array.from(root.querySelectorAll<HTMLElement>('.d2h-file-wrapper')).find(
                wrapper => wrapper.querySelector('.d2h-file-name')?.textContent?.trim() === name
            );
        const addedOf = (name: string) =>
            wrapperOf(name)?.querySelector('.diff-viewer__added')?.textContent;
        expect([addedOf('a.txt'), addedOf('logo.png'), addedOf('b.txt'), addedOf('c.txt')]).toEqual(
            ['+1', '+0', '+2', '+1']
        );
        expect(wrapperOf('logo.png')?.querySelectorAll('.diff-viewer__expand')).toHaveLength(0);

        wrapperOf('c.txt')?.querySelector<HTMLButtonElement>('.diff-viewer__expand')?.click();

        expect(loader).toHaveBeenCalledWith('c.txt', 'head-sha');
        expect(wrapperOf('c.txt')?.textContent).toContain('line 1');
        expect(wrapperOf('b.txt')?.textContent).not.toContain('line 1');
        expect(root.querySelectorAll('.d2h-file-wrapper')).toHaveLength(4);
    });

    it('does not offer expansion for a raw patch', () => {
        const fixture = TestBed.createComponent(DiffViewerComponent);
        fixture.componentRef.setInput('rawPatch', RAW_PATCH);
        fixture.componentRef.setInput('fileContentLoader', vi.fn());
        fixture.detectChanges();
        expect(fixture.nativeElement.querySelectorAll('.diff-viewer__expand')).toHaveLength(0);
    });
});
