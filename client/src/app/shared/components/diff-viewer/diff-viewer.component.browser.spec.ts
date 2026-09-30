import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { SharedModule } from '../../shared.module';
import { DiffViewerComponent } from './diff-viewer.component';
import { of } from 'rxjs';
import { FileContentLoader } from 'src/app/project/model/git-integration.model';

const CONTENT = Array.from({ length: 20 }, (_, i) => `line ${i + 1}`);

const STRUCTURED_PATCH = [
    '--- a/hello.txt',
    '+++ b/hello.txt',
    '@@ -10,3 +10,3 @@',
    ' context-10',
    '-old-11',
    '+new-11',
    ' context-12',
    '@@ -18,2 +18,2 @@',
    ' context-18',
    '-old-19',
    '+new-19'
].join('\n');

const RAW_PATCH = [
    '--- a/hello.txt',
    '+++ b/hello.txt',
    '@@ -1,2 +1,2 @@',
    ' keep',
    '-old line',
    '+new line'
].join('\n');

describe('DiffViewerComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [SharedModule, TranslateModule.forRoot()]
        }).compileComponents();
    });

    function setupRaw() {
        const fixture = TestBed.createComponent(DiffViewerComponent);
        fixture.componentRef.setInput('rawPatch', RAW_PATCH);
        fixture.detectChanges();
        const header = fixture.nativeElement.querySelector('.d2h-file-header') as HTMLElement;
        const body = fixture.nativeElement.querySelector('.d2h-file-diff') as HTMLElement;
        return { fixture, header, body };
    }

    function setupStructured() {
        const fixture = TestBed.createComponent(DiffViewerComponent);
        const loader: FileContentLoader = vi
            .fn()
            .mockReturnValue(of({ lines: CONTENT, lineCount: 20 }));
        fixture.componentRef.setInput('diff', {
            headSha: 'abc',
            baseSha: 'def',
            files: [
                {
                    oldPath: 'hello.txt',
                    newPath: 'hello.txt',
                    patch: STRUCTURED_PATCH,
                    isDeleted: false
                }
            ]
        });
        fixture.componentRef.setInput('fileContentLoader', loader);
        fixture.detectChanges();
        return { fixture, loader };
    }

    it('exposes the file header as a focusable, expanded button in raw mode', () => {
        const { header } = setupRaw();
        expect(header.getAttribute('role')).toBe('button');
        expect(header.getAttribute('tabindex')).toBe('0');
        expect(header.getAttribute('aria-expanded')).toBe('true');
    });

    it('collapses and re-expands the file on Enter in raw mode', () => {
        const { header, body } = setupRaw();

        header.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        expect(body.classList).toContain('d2h-d-none');
        expect(header.getAttribute('aria-expanded')).toBe('false');

        header.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        expect(body.classList).not.toContain('d2h-d-none');
        expect(header.getAttribute('aria-expanded')).toBe('true');
    });

    it('collapses on Space too in raw mode', () => {
        const { header, body } = setupRaw();
        header.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));
        expect(body.classList).toContain('d2h-d-none');
    });

    it('keeps aria-expanded in sync when toggled by mouse in raw mode', () => {
        const { header } = setupRaw();
        header.click();
        expect(header.getAttribute('aria-expanded')).toBe('false');
    });

    it('loads context above the first hunk when the up expander is clicked', () => {
        const { fixture } = setupStructured();

        const upButton = fixture.nativeElement.querySelector('.diff-viewer__expander button');
        upButton.click();
        fixture.detectChanges();
        expect(fixture.nativeElement.textContent).toContain('line 1');
    });

    it('fills the gap between two hunks with a single expand-all click', () => {
        const { fixture } = setupStructured();

        const expanders = fixture.nativeElement.querySelectorAll('.diff-viewer__expander button');
        // The up expander at hunk 0 is first, then the between-hunks down expander.
        const betweenExpander = expanders[1];
        betweenExpander.click();
        fixture.detectChanges();
        expect(fixture.nativeElement.textContent).toContain('line 13');
    });

    it('still uses diff2html for a raw patch and has no structured expanders', () => {
        const { fixture } = setupRaw();

        expect(fixture.nativeElement.querySelector('.d2h-file-wrapper')).not.toBeNull();
        expect(fixture.nativeElement.querySelectorAll('.diff-viewer__expander').length).toBe(0);
    });
});
