import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { TranslateModule, TranslateLoader } from '@ngx-translate/core';
import { Observable, of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { MrLinkPickerComponent } from './mr-link-picker.component';
import { GitIntegrationApi } from 'src/app/project/api/git-integration.api.service';
import { GitIntegrationRes, HostType } from 'src/app/project/model/git-integration.model';
import { UiModule } from 'src/app/ui/ui.module';
import { TablerIconStub } from 'src/testing/stubs';

class EmptyLoader implements TranslateLoader {
    public getTranslation(_lang: string): Observable<Record<string, string>> {
        return of({});
    }
}

function makeIntegration(id: number, name = `git-${id}`): GitIntegrationRes {
    return {
        idGitIntegration: id,
        idProject: 1,
        name,
        hostType: HostType.GitHub,
        baseUrl: '',
        repoPath: '',
        createdAt: '',
        updatedAt: ''
    };
}

function configure(list: GitIntegrationRes[], idGitIntegration: number | null | undefined) {
    const load$ = vi.fn(() => of(list));
    TestBed.configureTestingModule({
        declarations: [MrLinkPickerComponent],
        imports: [
            ReactiveFormsModule,
            TranslateModule.forRoot({
                loader: { provide: TranslateLoader, useClass: EmptyLoader }
            }),
            UiModule,
            TablerIconStub
        ],
        providers: [{ provide: GitIntegrationApi, useValue: { load$ } }, provideNoopAnimations()]
    });
    const fixture = TestBed.createComponent(MrLinkPickerComponent);
    fixture.componentRef.setInput('idProject', 1);
    fixture.componentRef.setInput('idGitIntegration', idGitIntegration);
    fixture.componentRef.setInput('mrId', null);
    fixture.detectChanges();
    return fixture;
}

describe('MrLinkPickerComponent — auto-select single integration', () => {
    let fixture: ComponentFixture<MrLinkPickerComponent>;

    afterEach(() => {
        fixture?.destroy();
    });

    it('auto-selects the only integration when no idGitIntegration input is given', () => {
        const only = makeIntegration(7);
        fixture = configure([only], null);
        const component = fixture.componentInstance as any;

        expect(component.form.controls.idGitIntegration.value).toBe(7);
        expect(component.selectedIntegrationId()).toBe(7);
    });

    it('does not auto-select when multiple integrations are available', () => {
        fixture = configure([makeIntegration(1), makeIntegration(2)], null);
        const component = fixture.componentInstance as any;

        expect(component.form.controls.idGitIntegration.value).toBeNull();
        expect(component.selectedIntegrationId()).toBeNull();
    });

    it('keeps the pre-selected idGitIntegration even when a single integration loads', () => {
        const only = makeIntegration(7);
        fixture = configure([only], 7);
        const component = fixture.componentInstance as any;

        expect(component.form.controls.idGitIntegration.value).toBe(7);
    });

    it('keeps the pre-selected idGitIntegration even when multiple integrations load', () => {
        fixture = configure([makeIntegration(1), makeIntegration(2)], 1);
        const component = fixture.componentInstance as any;

        expect(component.form.controls.idGitIntegration.value).toBe(1);
    });

    it('leaves the control null when zero integrations are available', () => {
        fixture = configure([], null);
        const component = fixture.componentInstance as any;

        expect(component.form.controls.idGitIntegration.value).toBeNull();
        expect(component.selectedIntegrationId()).toBeNull();
    });
});
