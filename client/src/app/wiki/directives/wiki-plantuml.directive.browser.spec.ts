import { Component, input } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { TranslateModule } from '@ngx-translate/core';
import { MarkdownModule } from 'ngx-markdown';
import { of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { PlantUmlApi } from 'src/app/shared/plantuml/plantuml.api.service';
import { PlantUmlDiagramComponent } from 'src/app/shared/plantuml/plantuml-diagram.component';
import { WikiPlantUmlDirective } from './wiki-plantuml.directive';

@Component({
    standalone: false,
    template: `
        <div markdown appWikiPlantUml [data]="body()"></div>
    `
})
class HostComponent {
    public readonly body = input('');
}

describe('WikiPlantUmlDirective (browser)', () => {
    const api = { render$: vi.fn().mockReturnValue(of({ svg: '<svg></svg>' })) };

    function configure(enabled: boolean) {
        api.render$.mockClear();
        return TestBed.configureTestingModule({
            declarations: [HostComponent, WikiPlantUmlDirective, PlantUmlDiagramComponent],
            imports: [MarkdownModule.forRoot(), TranslateModule.forRoot()],
            providers: [
                { provide: PlantUmlApi, useValue: api },
                { provide: SettingsStore, useValue: { isPlantUmlEnabled: () => enabled } }
            ]
        }).compileComponents();
    }

    async function render(body: string): Promise<ComponentFixture<HostComponent>> {
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentRef.setInput('body', body);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        return fixture;
    }

    it('replaces the fence with an app-plantuml-diagram when enabled', async () => {
        await configure(true);
        const fixture = await render('Intro\n\n```plantuml\nAlice -> Bob: hi\n```\n\nTail.');
        expect(fixture.nativeElement.querySelector('app-plantuml-diagram')).not.toBeNull();
        expect(fixture.nativeElement.querySelector('pre > code.language-plantuml')).toBeNull();
        expect(fixture.nativeElement.textContent).toContain('Intro');
    });

    it('leaves the markdown code fence untouched and never calls the api when disabled', async () => {
        await configure(false);
        const fixture = await render('```plantuml\nAlice -> Bob: hi\n```');
        expect(fixture.nativeElement.querySelector('pre > code.language-plantuml')).not.toBeNull();
        expect(fixture.nativeElement.querySelector('app-plantuml-diagram')).toBeNull();
        expect(api.render$).not.toHaveBeenCalled();
    });

    it('does not touch mermaid blocks', async () => {
        await configure(true);
        const fixture = await render('```mermaid\nflowchart TD\n    A --> B\n```');
        expect(fixture.nativeElement.querySelector('pre > code.language-mermaid')).not.toBeNull();
        expect(fixture.nativeElement.querySelector('app-plantuml-diagram')).toBeNull();
    });

    it('emits settled once the replaced diagram settles', async () => {
        await configure(true);
        const fixture = TestBed.createComponent(HostComponent);
        fixture.componentRef.setInput('body', '```plantuml\nA -> B\n```');
        const directive = fixture.debugElement
            .query(By.directive(WikiPlantUmlDirective))
            .injector.get(WikiPlantUmlDirective);
        const settled = vi.fn();
        directive.settled.subscribe(settled);
        fixture.detectChanges();
        await vi.waitFor(() => expect(settled).toHaveBeenCalledTimes(1));
    });
});
