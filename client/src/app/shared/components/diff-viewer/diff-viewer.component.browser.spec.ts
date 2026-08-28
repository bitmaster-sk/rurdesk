import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { SharedModule } from '../../shared.module';
import { DiffViewerComponent } from './diff-viewer.component';

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
