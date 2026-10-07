import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateLoader, TranslateModule } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { describe, expect, it } from 'vitest';
import { TablerIconStub } from 'src/testing/stubs';
import { WikiMergeChunkKind } from '../../constants/wiki-merge-chunk-kind.enum';
import { WikiMergeChunk } from '../../model/wiki-save.model';
import { WikiConflictComponent } from './wiki-conflict.component';

@Component({ selector: 'ui-button', template: '<ng-content></ng-content>', standalone: true })
class ButtonStub {
    public readonly variant = input<string>('filled');
    public readonly size = input<string>('default');
    public readonly severity = input<string>('primary');
    public readonly disabled = input(false);
    public readonly loading = input(false);
}

@Component({ selector: 'ui-tag', template: '{{ value() }}', standalone: true })
class TagStub {
    public readonly value = input<string>('');
    public readonly severity = input<string>('secondary');
}

class EmptyLoader implements TranslateLoader {
    public getTranslation(): Observable<Record<string, string>> {
        return of({});
    }
}

const chunks: WikiMergeChunk[] = [
    { kind: WikiMergeChunkKind.Stable, lines: ['intro'] },
    { kind: WikiMergeChunkKind.Conflict, base: ['shared'], mine: ['mine'], theirs: ['theirs'] },
    { kind: WikiMergeChunkKind.Stable, lines: ['outro'] }
];

async function setup(): Promise<{
    element: HTMLElement;
    emitted: string[];
    fixture: ComponentFixture<WikiConflictComponent>;
}> {
    await TestBed.configureTestingModule({
        declarations: [WikiConflictComponent],
        imports: [
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            ButtonStub,
            TagStub,
            TablerIconStub
        ]
    }).compileComponents();
    const fixture = TestBed.createComponent(WikiConflictComponent);
    fixture.componentRef.setInput('chunks', chunks);
    fixture.componentRef.setInput('theirsLabel', 'Robert · v8');
    const emitted: string[] = [];
    fixture.componentInstance.resolved.subscribe(text => emitted.push(text));
    fixture.detectChanges();
    return { element: fixture.nativeElement as HTMLElement, emitted, fixture };
}

describe('WikiConflictComponent', () => {
    it('shows both sides of a conflict next to the merged context', async () => {
        const { element } = await setup();
        expect(element.querySelectorAll('[data-testid="wiki-conflict-chunk"]').length).toBe(1);
        expect(element.textContent).toContain('intro');
        expect(element.textContent).toContain('mine');
        expect(element.textContent).toContain('theirs');
        expect(element.textContent).toContain('Robert · v8');
    });

    it('does not save until every conflict has a choice', async () => {
        const { element, emitted, fixture } = await setup();
        const save = element.querySelector<HTMLElement>('[data-testid="wiki-conflict-save"]');
        save?.click();
        expect(emitted).toEqual([]);
        expect(save?.textContent).toContain('WIKI.CONFLICT.REMAINING');

        element.querySelector<HTMLElement>('[data-testid="wiki-conflict-theirs"]')?.click();
        fixture.detectChanges();
        expect(save?.textContent).toContain('WIKI.CONFLICT.SAVE');
    });

    it('emits the text composed from the chosen side', async () => {
        const { element, emitted, fixture } = await setup();
        element.querySelector<HTMLElement>('[data-testid="wiki-conflict-theirs"]')?.click();
        fixture.detectChanges();
        element.querySelector<HTMLElement>('[data-testid="wiki-conflict-save"]')?.click();
        expect(emitted).toEqual(['intro\ntheirs\noutro']);
    });
});
