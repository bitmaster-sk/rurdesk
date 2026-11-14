import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import {
    IconArrowRight,
    IconInfoCircle,
    IconTrash,
    IconUsersGroup,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { TeamLabelComponent } from 'src/app/shared/team-label/team-label.component';
import { UiModule } from 'src/app/ui/ui.module';
import { SsoHintComponent } from '../sso-hint/sso-hint.component';
import { SsoGroupMappingComponent } from './sso-group-mapping.component';

describe('SsoGroupMappingComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SsoGroupMappingComponent, TeamLabelComponent, SsoHintComponent],
            imports: [
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconComponent
            ],
            providers: [
                provideTablerIcons({ IconUsersGroup, IconTrash, IconArrowRight, IconInfoCircle })
            ]
        }).compileComponents();
    });

    function setup() {
        const fixture = TestBed.createComponent(SsoGroupMappingComponent);
        fixture.componentRef.setInput('mappings', [
            { idSsoGroupMapping: 4, groupValue: 'dev', idTeam: 1, teamName: 'Developers' }
        ]);
        fixture.componentRef.setInput('teams', [
            { idTeam: 1, name: 'Developers', color: '#fff' },
            { idTeam: 2, name: 'Operations', color: '#000' }
        ]);
        fixture.componentRef.setInput('suggestedGroups', ['dev', 'ops']);
        fixture.detectChanges();
        const add = vi.fn();
        const remove = vi.fn();
        fixture.componentInstance.add.subscribe(add);
        fixture.componentInstance.remove.subscribe(remove);
        return { fixture, add, remove };
    }

    function all(testId: string): HTMLElement[] {
        return Array.from(document.querySelectorAll(`[data-testid="${testId}"]`));
    }

    it('lists the mappings and removes one', () => {
        const { remove } = setup();

        expect(all('sso-mapping-row')).toHaveLength(1);
        expect(all('sso-mapping-row')[0].textContent).toContain('Developers');
        const dot = all('sso-mapping-row')[0].querySelector(
            '[data-testid="team-label-dot"]'
        ) as HTMLElement;
        expect(dot.style.backgroundColor).toBe('rgb(255, 255, 255)');

        (all('sso-mapping-remove')[0].querySelector('button') as HTMLButtonElement).click();

        expect(remove).toHaveBeenCalledWith(4);
    });

    it('offers the unmapped groups from the last test', () => {
        const { fixture } = setup();

        const suggestions = all('sso-mapping-suggestion');
        expect(suggestions.map(s => s.textContent?.trim())).toEqual(['ops']);

        (suggestions[0].querySelector('button') as HTMLButtonElement).click();
        fixture.detectChanges();

        expect((all('sso-mapping-group')[0] as HTMLInputElement).value).toBe('ops');
    });

    it('adds a mapping once the group and the team are filled in', () => {
        const { fixture, add } = setup();
        const internals = fixture.componentInstance as unknown as {
            form: SsoGroupMappingComponent['form'];
            onAdd(): void;
        };

        internals.onAdd();
        expect(add).not.toHaveBeenCalled();

        internals.form.setValue({ groupValue: ' ops ', idTeam: 2 });
        internals.onAdd();

        expect(add).toHaveBeenCalledWith({ groupValue: 'ops', idTeam: 2 });
        expect(internals.form.getRawValue()).toEqual({ groupValue: '', idTeam: null });
    });

    it('shows the picked team with its color', () => {
        const { fixture } = setup();
        const internals = fixture.componentInstance as unknown as {
            form: SsoGroupMappingComponent['form'];
        };

        internals.form.controls.idTeam.setValue(2);
        fixture.detectChanges();

        const picked = all('sso-mapping-team')[0];
        expect(picked.querySelector('[data-testid="team-label-name"]')?.textContent).toBe(
            'Operations'
        );
        const dot = picked.querySelector('[data-testid="team-label-dot"]') as HTMLElement;
        expect(dot.style.backgroundColor).toBe('rgb(0, 0, 0)');
    });
});
